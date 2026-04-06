package conf

import (
	"encoding/json"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/socks"
	"google.golang.org/protobuf/proto"
)

type SocksAccount struct {
	Username string `json:"user"`
	Password string `json:"pass"`
}

func (v *SocksAccount) Build() *socks.Account {
	return &socks.Account{
		Username: v.Username,
		Password: v.Password,
	}
}

const (
	AuthMethodNoAuth   = "noauth"
	AuthMethodUserPass = "password"
)

type SocksServerConfig struct {
	AuthMethod string          `json:"auth"`
	Accounts   []*SocksAccount `json:"accounts"`
	UDP        bool            `json:"udp"`
	Host       *Address        `json:"ip"`
	UserLevel  uint32          `json:"userLevel"`
}

func (v *SocksServerConfig) Build() (proto.Message, error) {
	config := new(socks.ServerConfig)
	switch v.AuthMethod {
	case AuthMethodNoAuth:
		config.AuthType = socks.AuthType_NO_AUTH
	case AuthMethodUserPass:
		config.AuthType = socks.AuthType_PASSWORD
	default:
		// errors.New("unknown socks auth method: ", v.AuthMethod, ". Default to noauth.").AtWarning().WriteToLog()
		config.AuthType = socks.AuthType_NO_AUTH
	}

	if len(v.Accounts) > 0 {
		config.Accounts = make(map[string]string, len(v.Accounts))
		for _, account := range v.Accounts {
			config.Accounts[account.Username] = account.Password
		}
	}

	config.UdpEnabled = v.UDP
	if v.Host != nil {
		config.Address = v.Host.Build()
	}

	config.UserLevel = v.UserLevel
	return config, nil
}

type SocksRemoteConfig struct {
	Address *Address          `json:"address"`
	Port    uint16            `json:"port"`
	Users   []json.RawMessage `json:"users"`
}

type SocksClientConfig struct {
	Address  *Address             `json:"address"`
	Port     uint16               `json:"port"`
	Level    uint32               `json:"level"`
	Email    string               `json:"email"`
	Username string               `json:"user"`
	Password string               `json:"pass"`
	Servers  []*SocksRemoteConfig `json:"servers"`
}

func (v *SocksClientConfig) Build() (proto.Message, error) {
	config := new(socks.ClientConfig)
	if v.Address != nil {
		v.Servers = []*SocksRemoteConfig{
			{
				Address: v.Address,
				Port:    v.Port,
			},
		}
		if len(v.Username) > 0 {
			v.Servers[0].Users = []json.RawMessage{{}}
		}
	}
	if len(v.Servers) != 1 {
		return nil, errors.New(`SOCKS settings: "servers" should have one and only one member. Multiple endpoints in "servers" should use multiple SOCKS outbounds and routing balancer instead`)
	}
	for _, serverConfig := range v.Servers {
		if len(serverConfig.Users) > 1 {
			return nil, errors.New(`SOCKS servers: "users" should have one member at most. Multiple members in "users" should use multiple SOCKS outbounds and routing balancer instead`)
		}
		server := &protocol.ServerEndpoint{
			Address: serverConfig.Address.Build(),
			Port:    uint32(serverConfig.Port),
		}
		for _, rawUser := range serverConfig.Users {
			user := new(protocol.User)
			if v.Address != nil {
				user.Level = v.Level
				user.Email = v.Email
			} else {
				if err := json.Unmarshal(rawUser, user); err != nil {
					return nil, errors.New("failed to parse Socks user").Base(err).AtError()
				}
			}
			account := new(SocksAccount)
			if v.Address != nil {
				account.Username = v.Username
				account.Password = v.Password
			} else {
				if err := json.Unmarshal(rawUser, account); err != nil {
					return nil, errors.New("failed to parse socks account").Base(err).AtError()
				}
			}
			user.Account = serial.ToTypedMessage(account.Build())
			server.User = user
			break
		}
		config.Server = server
		break
	}
	return config, nil
}
