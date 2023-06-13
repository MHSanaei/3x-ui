package xray

import (
	"context"
	"fmt"
	"regexp"
	"time"
	"x-ui/util/common"

	"github.com/xtls/xray-core/app/proxyman/command"
	statsService "github.com/xtls/xray-core/app/stats/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vmess"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type XrayAPI struct {
	HandlerServiceClient *command.HandlerServiceClient
	StatsServiceClient   *statsService.StatsServiceClient
	grpcClient           *grpc.ClientConn
	isConnected          bool
}

func (x *XrayAPI) Init(apiPort int) (err error) {
	if apiPort == 0 {
		return common.NewError("xray api port wrong:", apiPort)
	}
	x.grpcClient, err = grpc.Dial(fmt.Sprintf("127.0.0.1:%v", apiPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	x.isConnected = true

	hsClient := command.NewHandlerServiceClient(x.grpcClient)
	ssClient := statsService.NewStatsServiceClient(x.grpcClient)

	x.HandlerServiceClient = &hsClient
	x.StatsServiceClient = &ssClient

	return
}

func (x *XrayAPI) Close() {
	x.grpcClient.Close()
	x.HandlerServiceClient = nil
	x.StatsServiceClient = nil
	x.isConnected = false
}

func (x *XrayAPI) AddUser(Protocol string, inboundTag string, user map[string]interface{}) error {
	var account *serial.TypedMessage
	switch Protocol {
	case "vmess":
		account = serial.ToTypedMessage(&vmess.Account{
			Id: user["id"].(string),
		})
	case "vless":
		account = serial.ToTypedMessage(&vless.Account{
			Id:   user["id"].(string),
			Flow: user["flow"].(string),
		})
	case "trojan":
		account = serial.ToTypedMessage(&trojan.Account{
			Password: user["password"].(string),
		})
	case "shadowsocks":
		account = serial.ToTypedMessage(&shadowsocks.Account{
			Password: user["password"].(string),
		})
	default:
		return nil
	}

	client := *x.HandlerServiceClient

	_, err := client.AlterInbound(context.Background(), &command.AlterInboundRequest{
		Tag: inboundTag,
		Operation: serial.ToTypedMessage(&command.AddUserOperation{
			User: &protocol.User{
				Email:   user["email"].(string),
				Account: account,
			},
		}),
	})
	return err
}

func (x *XrayAPI) RemoveUser(inboundTag string, email string) error {
	client := *x.HandlerServiceClient
	_, err := client.AlterInbound(context.Background(), &command.AlterInboundRequest{
		Tag: inboundTag,
		Operation: serial.ToTypedMessage(&command.RemoveUserOperation{
			Email: email,
		}),
	})
	return err
}

func (x *XrayAPI) GetTraffic(reset bool) ([]*Traffic, []*ClientTraffic, error) {
	if x.grpcClient == nil {
		return nil, nil, common.NewError("xray api is not initialized")
	}
	var trafficRegex = regexp.MustCompile("(inbound|outbound)>>>([^>]+)>>>traffic>>>(downlink|uplink)")
	var ClientTrafficRegex = regexp.MustCompile("(user)>>>([^>]+)>>>traffic>>>(downlink|uplink)")

	client := *x.StatsServiceClient
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	request := &statsService.QueryStatsRequest{
		Reset_: reset,
	}
	resp, err := client.QueryStats(ctx, request)
	if err != nil {
		return nil, nil, err
	}
	tagTrafficMap := map[string]*Traffic{}
	emailTrafficMap := map[string]*ClientTraffic{}

	clientTraffics := make([]*ClientTraffic, 0)
	traffics := make([]*Traffic, 0)
	for _, stat := range resp.GetStat() {
		matchs := trafficRegex.FindStringSubmatch(stat.Name)
		if len(matchs) < 3 {

			matchs := ClientTrafficRegex.FindStringSubmatch(stat.Name)
			if len(matchs) < 3 {
				continue
			} else {

				isUser := matchs[1] == "user"
				email := matchs[2]
				isDown := matchs[3] == "downlink"
				if !isUser {
					continue
				}
				traffic, ok := emailTrafficMap[email]
				if !ok {
					traffic = &ClientTraffic{
						Email: email,
					}
					emailTrafficMap[email] = traffic
					clientTraffics = append(clientTraffics, traffic)
				}
				if isDown {
					traffic.Down = stat.Value
				} else {
					traffic.Up = stat.Value
				}

			}
			continue
		}
		isInbound := matchs[1] == "inbound"
		tag := matchs[2]
		isDown := matchs[3] == "downlink"
		if tag == "api" {
			continue
		}
		traffic, ok := tagTrafficMap[tag]
		if !ok {
			traffic = &Traffic{
				IsInbound: isInbound,
				Tag:       tag,
			}
			tagTrafficMap[tag] = traffic
			traffics = append(traffics, traffic)
		}
		if isDown {
			traffic.Down = stat.Value
		} else {
			traffic.Up = stat.Value
		}
	}

	return traffics, clientTraffics, nil
}
