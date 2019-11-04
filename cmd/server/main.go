package main

import (
	"encoding/json"
	"github.com/ovandriyanov/tgpunch/pkg/common"
	"github.com/ovandriyanov/tgpunch/pkg/tgapi"
	"net/http"
    "errors"
    "fmt"
    "os"
    "strings"
    "net"
)

func handleHubMessage(client *http.Client, config *common.Config, msg *common.HubMessage) error {
	fmt.Println("Handling hub message")
	switch msg.Type {
	case "start_punching_request":
		return handleStartPunchingRequest(client, config, msg.PublicEndpoint, msg.Serial)

	default:
		fmt.Println("Unknown message type: " + msg.Type)
	}
	return nil
}

func handleStartPunchingRequest(client *http.Client, config *common.Config, clientEndpoint *common.Endpoint, serial uint64) error {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0})
	if err != nil {
		return err
	}
	defer conn.Close()

	myEndpoint, err := common.GetMyPublicEndpoint(conn, config)
	if err != nil {
		return err
	}
	fmt.Printf("My public endpoint is %v\n", myEndpoint)

	// Send the message with our public endpoint to the hub

	innerJsonMessage, err := json.Marshal(&common.HubMessage{
		Type: "start_punching_response",
		Serial: serial,
		PublicEndpoint: &myEndpoint,
	})

	if err != nil {
		return err
	}

	response, err := client.Post(
        common.ApiUrlPrefix + config.ApiToken + "/sendMessage",
        "application/json",
		strings.NewReader(
			fmt.Sprintf(
				`{ "chat_id": %d, "text": %s }`,
				config.ChatId,
				string(innerJsonMessage),
			),
		),
	)
	if err != nil {
		return err
	}

	var apiResponse tgapi.SendMessageResponse
	if err = json.NewDecoder(response.Body).Decode(&apiResponse); err != nil {
		return err
	}

	if !apiResponse.Ok {
		return errors.New(common.ApiErrorDescription(apiResponse.Description))
	}

	remoteAddr := net.UDPAddr{
		IP: net.ParseIP(clientEndpoint.Address),
		Port: clientEndpoint.Port,
	}

	err = common.PunchHole(conn, &remoteAddr, []byte("server"), []byte("server"))
	if err != nil {
		common.Fatal(err.Error())
	}

	return nil
}

func main() {
    config, err := common.ParseCmdLine(os.Args[1:])
    if err != nil {
        common.Fatal("Cannot parse command line: " + err.Error())
    }

    client := common.MakeClient(*config)

    // First of all try sending getMe request to test if bot is working

    if err = common.TestBotWithGetMe(client, config); err != nil {
        common.Fatal("getMe call failed: " + err.Error())
    }
    fmt.Println("getMe works")

	// Get the ID of the last update just to ignore all the previous updates

	updateOffset, err := common.GetLastUpdateId(client, config)
	if err != nil {
		common.Fatal("Cannot get last update ID: " + err.Error())
	}
	fmt.Printf("Last update ID is %d\n", updateOffset)

    // Start receiving updates from the Telegram channel given on the command line

    for {
		updates, err := common.GetUpdates(client, config, updateOffset)
		if err != nil {
			common.Fatal("Cannot get updates from server: " + err.Error())
		}
		for _, upd := range(updates) {
			if upd.ChannelPost == nil {
				continue
			}
			if upd.ChannelPost.Chat.Id != config.ChatId {
				continue
			}
			if upd.ChannelPost.Text == nil {
				continue
			}
			fmt.Println("Got channel message: " + *upd.ChannelPost.Text)

			var msg common.HubMessage
			if err = json.Unmarshal([]byte(*upd.ChannelPost.Text), &msg); err != nil {
				fmt.Println("Cannot parse hub message: " + err.Error())
				continue
			}

			if err = handleHubMessage(client, config, &msg); err != nil {
				common.Fatal("Cannot handle hub message: " + err.Error())
			}
		}

		if len(updates) > 0 {
			updateOffset = updates[len(updates) - 1].Id
		}
    }
}
