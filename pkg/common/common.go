package common

import (
	"encoding/json"
	"github.com/ovandriyanov/tgpunch/pkg/stun"
	"github.com/ovandriyanov/tgpunch/pkg/tgapi"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
    "bytes"
    "errors"
    "fmt"
    "os"
)

const ApiUrlPrefix = "https://api.telegram.org/bot"

type Config struct {
    ApiToken string
    ChatId int64
    ProxyUrl *url.URL
}

type HubMessage struct {
	Type string `json:"type"`
	Serial uint64 `json:"serial"`
	PublicEndpoint *Endpoint `json:"public_endpoint"`
}

type Endpoint struct {
	Address string `json:"address"`
	Port int `json:"port"`
}

func Fatal(message string) {
    fmt.Fprintf(os.Stderr, "Error: %v\n", message)
    os.Exit(1)
}

func ParseCmdLine(args []string) (*Config, error) {
    var apiToken *string
    var chatId *int64
    var proxyUrl *url.URL
    var err error

    for arg := 0; arg < len(args); arg++ {
        switch {
        case args[arg] == "-t" || args[arg] == "--api-token":
            arg++
            if arg >= len(args) {
                return nil, errors.New("--api-token requires a string argument")
            }
            apiToken = &args[arg]
        case args[arg] == "-c" || args[arg] == "--chat":
            arg++
            if arg >= len(args) {
                return nil, errors.New("--chat requires an integer argument")
            }
			chatId = new(int64)
			*chatId, err = strconv.ParseInt(args[arg], 10, 64)
			if err != nil {
				return nil, errors.New("Cannot parse chat: " + err.Error())
			}
        case args[arg] == "-x" || args[arg] == "--proxy":
            arg++
            if arg >= len(args) {
                return nil, errors.New("--proxy requires a string argument")
            }
            proxyUrl, err = url.Parse(args[arg])
            if err != nil {
                return nil, errors.New("Cannot parse proxy URL: " + err.Error())
            }
        }
    }

    // Check required arguments
    if apiToken == nil {
        return nil, errors.New("No API token given on the command line")
    }

    if chatId == nil {
        return nil, errors.New("No chat given on the command line")
    }

    return &Config{
        ApiToken: *apiToken,
        ChatId: *chatId,
        ProxyUrl: proxyUrl,
    }, nil
}

func MakeClient(config Config) *http.Client {
    return &http.Client{
        Transport: &http.Transport{
            Proxy: http.ProxyURL(config.ProxyUrl),
        },
    }
}

func ApiErrorDescription(description *string) string {
	if description != nil {
		return *description
	}
	return "Unknown error"
}

func ToJsonReader(object interface {}) *bytes.Reader {
	json, err := json.Marshal(object)
	if err != nil {
		panic("Cannot serialize getUpdates request: " + err.Error())
	}
	return bytes.NewReader(json)
}

func TestBotWithGetMe(client *http.Client, config *Config) error {
    response, err := client.Get(ApiUrlPrefix + config.ApiToken + "/getMe")
    if err != nil {
        return err
    }

    var apiResponse tgapi.GetMeResponse
    err = json.NewDecoder(response.Body).Decode(&apiResponse)
    if err != nil {
        return errors.New("Cannot decode JSON response from server: " + err.Error())
    }

    if !apiResponse.Ok {
		return errors.New(ApiErrorDescription(apiResponse.Description))
    }

    return nil
}

func GetLastUpdateId(client *http.Client, config *Config) (int, error) {
	response, err := client.Post(
        ApiUrlPrefix + config.ApiToken + "/getUpdates",
        "application/json",
        ToJsonReader(&tgapi.GetUpdates{
			Offset: -1,
			Timeout: 0,
		}),
    )
	if err != nil {
		return 0, err
	}
	var apiResponse tgapi.GetUpdatesResponse
	err = json.NewDecoder(response.Body).Decode(&apiResponse)
	if err != nil {
		return 0, err
	}
	if !apiResponse.Ok {
		return 0, errors.New(ApiErrorDescription(apiResponse.Description))
	}
	if len(apiResponse.Result) == 0 {
		return 0, nil
	}
	return apiResponse.Result[len(apiResponse.Result) - 1].Id, nil
}

func GetUpdates(client *http.Client, config *Config, offset int) ([]tgapi.Update, error) {
	response, err := client.Post(
        ApiUrlPrefix + config.ApiToken + "/getUpdates",
        "application/json",
        ToJsonReader(&tgapi.GetUpdates{
			Offset: offset + 1,
			Timeout: 30,
			AllowedUpdates: []string{"channel_post"},
		}),
    )
	if err != nil {
		return nil, err
	}

	var apiResponse tgapi.GetUpdatesResponse
	err = json.NewDecoder(response.Body).Decode(&apiResponse)
	if err != nil {
		return nil, err
	}
	if !apiResponse.Ok {
		return nil, errors.New(ApiErrorDescription(apiResponse.Description))
	}

	return apiResponse.Result, nil
}

func GetMyPublicEndpoint(conn *net.UDPConn, config *Config) (Endpoint, error) {
	serverAddress := net.UDPAddr{IP: net.ParseIP("109.71.104.73"), Port: 3478}
	transactionId, err := stun.SendBindingRequest(conn, &serverAddress)
	if err != nil {
		return Endpoint{}, err
	}

	addrChan := make(chan *net.UDPAddr)
	errChan := make(chan error)
	retryChan := time.After(1 * time.Second)

	go func() {
		address, err := stun.ReceiveBindingResponse(conn, &serverAddress, transactionId)
		if err != nil {
			errChan <-err
			return
		}
		addrChan <-address
	}()

	retries := 0
	maxRetries := 4
	for {
		select {
		case addr := <-addrChan:
			return Endpoint{
				Address: addr.IP.String(),
				Port: addr.Port,
			}, nil

		case err := <-errChan:
			return Endpoint{}, err

		case <-retryChan:
			if retries == maxRetries {
				return Endpoint{}, errors.New(fmt.Sprintf("No response from server after %d retries", retries))
			}

			retries++
			_, err := stun.SendBindingRequest(conn, &serverAddress)
			if err != nil {
				return Endpoint{}, err
			}
			retryChan = time.After(1 * time.Second)
		}
	}
}
