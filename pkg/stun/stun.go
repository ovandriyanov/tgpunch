package stun

import (
	"net"
	"fmt"
	"bytes"
	"encoding/binary"
	"errors"
)

const (
	magicCookie             = 0x2112A442
	stunHeaderLen           = 20
	familyIpv4              = 0x01
	familyIpv6              = 0x02
)

func makeBindingRequest() []byte {
	var buffer [stunHeaderLen]byte

	//  0                   1                   2                   3
	//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |0 0|     STUN Message Type     |         Message Length        |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |                         Magic Cookie                          |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |                                                               |
	// |                     Transaction ID (96 bits)                  |
	// |                                                               |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	// The binding request method and class is encoded as 0x0001
	binary.BigEndian.PutUint16(buffer[0:], 0x0001)

	// The message length for binding request is 0 (header only, no attributes)

	// Put magic cookie
	binary.BigEndian.PutUint32(buffer[4:], magicCookie)

	// Transaction ID is hardcoded for now
	// TODO: generate randomly
	copy(buffer[8:], []byte("Hello, world"))

	return buffer[:]
}

func isReservedAttribute(attrType uint16) bool {
	return attrType == 0x0000 ||
		attrType == 0x0002 ||
		attrType == 0x0003 ||
		attrType == 0x0004 ||
		attrType == 0x0005 ||
		attrType == 0x0007 ||
		attrType == 0x000b
}

func xorSlice(lhs []byte, rhs []byte) []byte {
	var length int
	if len(lhs) < len(rhs) {
		length = len(lhs)
	} else {
		length = len(rhs)
	}
	result := make([]byte, length)

	for i := 0; i < length; i++ {
		result[i] = lhs[i] ^ rhs[i]
	}

	return result
}

func extractAddressFromBindingResponse(response []byte) (*net.UDPAddr, error) {
	// Example of server response:
	// 00000000  01 01 00 4c 21 12 a4 42  48 65 6c 6c 6f 2c 20 77  |...L!..BHello, w|
	// 00000010  6f 72 6c 64 00 04 00 08  00 01 0d 96 6d 47 68 49  |orld........mGhI|
	// 00000020  00 05 00 08 00 01 0d 97  6d 47 68 4c 00 20 00 08  |........mGhL. ..|
	// 00000030  00 01 ba ff 24 00 56 02  80 22 00 24 72 65 54 55  |....$.V..".$reTU|
	// 00000040  52 4e 53 65 72 76 65 72  20 31 2e 31 31 2e 30 7e  |RNServer 1.11.0~|
	// 00000050  62 65 74 61 31 20 28 52  46 43 35 33 38 39 29 20  |beta1 (RFC5389) |

	if len(response) < stunHeaderLen {
		return nil, errors.New("STUN message header truncated")
	}

	first16bits := binary.BigEndian.Uint16(response[0:])

	if first16bits & 0xc000 != 0 {
		return nil, errors.New("STUN message header malformed: first two bits are not zero")
	}

	receivedCookie := binary.BigEndian.Uint32(response[4:])
	if receivedCookie != magicCookie {
		return nil, errors.New("STUN message header malformed: magic cookie mismatch")
	}

	messageLengthInHeader := int(binary.BigEndian.Uint16(response[2:]))
	actualMessageLength := len(response) - stunHeaderLen
	if actualMessageLength < messageLengthInHeader {
		return nil, errors.New("Incomplete STUN message")
	}
	if actualMessageLength > messageLengthInHeader {
		return nil, errors.New("Trailing data")
	}

	// TODO: receive session ID as an argument
	var transactionId [12]byte
	copy(transactionId[:], response[8:20])
	if bytes.Compare(transactionId[:], []byte("Hello, world")) != 0 {
		return nil, errors.New("Transaction ID mismatch")
	}

	// Now parse attributes

	// 0                   1                   2                   3
	// 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	// -+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//          Type                  |            Length             |
	// -+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//                          Value (variable)                ....
	// -+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	var address *net.UDPAddr
	payload := response[stunHeaderLen:]
	for len(payload) > 0 {
		attrType := binary.BigEndian.Uint16(payload[0:])
		attrLen := int(binary.BigEndian.Uint16(payload[2:]))
		payload = payload[4:]
		if attrLen > len(payload) {
			return nil, errors.New("Attribute truncated")
		}

		// Skip comprehension-optional attributes
		if attrType & 0x8000 > 0 {
			payload = payload[attrLen:]
			continue
		}

		// Also skip reserved attributes
		if isReservedAttribute(attrType) {
			payload = payload[attrLen:]
			continue
		}

		// We are going to handle only XOR-MAPPED-ADDRESS attribute (0x0020)
		if attrType != 0x0020 {
			return nil, errors.New(fmt.Sprintf("Unknown comprehension-required attribute: %x", attrType))
		}

		// XOR-MAPPED-ADDRESS attribute structure:

		//  0                   1                   2                   3
		//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
		// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		// |x x x x x x x x|    Family     |         X-Port                |
		// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		// |                X-Address (Variable)
		// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

		family := payload[1]
		if family == familyIpv4 {
			if attrLen != 8 {
				return nil, errors.New(fmt.Sprintf("Invalid attribute length for IPv4 address: %d", attrLen))
			}
		} else if family == familyIpv6 {
			if attrLen != 20 {
				return nil, errors.New(fmt.Sprintf("Invalid attribute length for IPv6 address: %d", attrLen))
			}
		}

		xPort := binary.BigEndian.Uint16(payload[2:])
		xAddress := payload[4:attrLen]
		address = &net.UDPAddr{
			IP: xorSlice(xAddress, response[4:20]),
			Port: int(xPort ^ ((magicCookie >> 16) & 0xffff)),
		}

		payload = payload[attrLen:]
	}

	if address == nil {
		return nil, errors.New("No XOR-MAPPED-ADDRESS attribute found in STUN message")
	}

	return address, nil
}

func GetReflexiveAddress(conn *net.UDPConn, serverAddress *net.UDPAddr) (*net.UDPAddr, error) {
	request := makeBindingRequest()
	nwritten, err := conn.WriteToUDP(request, serverAddress)
	if err != nil {
		return nil, err
	}
	if nwritten != len(request) {
		return nil, errors.New("Outbound datagram truncated")
	}

	var buffer [4096]byte
	var nread int
	for {
		var addr *net.UDPAddr
		var err error

		nread, addr, err = conn.ReadFromUDP(buffer[:])
		if err != nil {
			return nil, err
		}
		if bytes.Compare(addr.IP, serverAddress.IP) == 0 && addr.Port == serverAddress.Port {
			break
		}
	}
	if nread >= len(buffer) {
		return nil, errors.New("A packet from server is too large")
	}

	return extractAddressFromBindingResponse(buffer[:nread])
}
