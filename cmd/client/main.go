package main

import (
	"encoding/json"
	"github.com/ovandriyanov/tgpunch/pkg/common"
	"github.com/ovandriyanov/tgpunch/pkg/tgapi"
	"math/rand"
	"strings"
	"time"
    "fmt"
    "net"
    "os"
)

func main() {
	rand.Seed(time.Now().UnixNano())

    config, err := common.ParseCmdLine(os.Args[1:])
    if err != nil {
        common.Fatal("Cannot parse command line: " + err.Error())
    }

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0})
	if err != nil {
		common.Fatal("Cannot create UDP socket: " + err.Error())
	}
	defer conn.Close()

	myEndpoint, err := common.GetMyPublicEndpoint(conn, config)
	if err != nil {
		common.Fatal("Cannot get my public endpoint: " + err.Error())
	}
	fmt.Printf("Our public endpoint is %v\n", myEndpoint)

    client := common.MakeClient(*config)

	serial := rand.Uint64()
	innerJsonMessage, err := json.Marshal(&common.HubMessage{
		Type: "start_punching_request",
		Serial: serial,
		PublicEndpoint: &myEndpoint,
	})

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
		common.Fatal("Cannot send chat message: " + err.Error())
	}

	var apiResponse tgapi.SendMessageResponse
	if err = json.NewDecoder(response.Body).Decode(&apiResponse); err != nil {
		common.Fatal("Cannot send chat message: " + err.Error())
	}
	fmt.Println("Sent start_punching_request to the chat")

	updateOffset := 0
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

			if msg.Type != "start_punching_response" {
				fmt.Printf("Unexpected hub message type: %s\n", msg.Type)
				continue
			}

			if msg.Serial != serial {
				fmt.Printf("Ignoring message with unexpected serial %d\n", msg.Serial)
				continue
			}

			fmt.Printf("Remote public endpoint is %v\n", *msg.PublicEndpoint)
			os.Exit(0)
		}

		if len(updates) > 0 {
			updateOffset = updates[len(updates) - 1].Id
		}
    }
}
