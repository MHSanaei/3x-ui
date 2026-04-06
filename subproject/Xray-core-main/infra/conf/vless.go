package conf

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/common/uuid"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vless/inbound"
	"github.com/xtls/xray-core/proxy/vless/outbound"
	"google.golang.org/protobuf/proto"
)

type VLessInboundFallback struct {
	Name string          `json:"name"`
	Alpn string          `json:"alpn"`
	Path string          `json:"path"`
	Type string          `json:"type"`
	Dest json.RawMessage `json:"dest"`
	Xver uint64          `json:"xver"`
}

type VLessInboundConfig struct {
	Clients    []json.RawMessage       `json:"clients"`
	Decryption string                  `json:"decryption"`
	Fallbacks  []*VLessInboundFallback `json:"fallbacks"`
	Flow       string                  `json:"flow"`
	Testseed   []uint32                `json:"testseed"`
}

// Build implements Buildable
func (c *VLessInboundConfig) Build() (proto.Message, error) {
	config := new(inbound.Config)
	config.Clients = make([]*protocol.User, len(c.Clients))
	switch c.Flow {
	case vless.XRV, "":
	default:
		return nil, errors.New(`VLESS "settings.flow" doesn't support "` + c.Flow + `" in this version`)
	}
	for idx, rawUser := range c.Clients {
		user := new(protocol.User)
		if err := json.Unmarshal(rawUser, user); err != nil {
			return nil, errors.New(`VLESS clients: invalid user`).Base(err)
		}
		account := new(vless.Account)
		if err := json.Unmarshal(rawUser, account); err != nil {
			return nil, errors.New(`VLESS clients: invalid user`).Base(err)
		}

		u, err := uuid.ParseString(account.Id)
		if err != nil {
			return nil, err
		}
		account.Id = u.String()

		switch account.Flow {
		case "":
			account.Flow = c.Flow
		case vless.XRV:
		default:
			return nil, errors.New(`VLESS clients: "flow" doesn't support "` + account.Flow + `" in this version`)
		}

		if len(account.Testseed) < 4 {
			account.Testseed = c.Testseed
		}

		if account.Encryption != "" {
			return nil, errors.New(`VLESS clients: "encryption" should not be in inbound settings`)
		}

		if account.Reverse != nil {
			if account.Reverse.Tag == "" {
				return nil, errors.New(`VLESS clients: "tag" can't be empty for "reverse"`)
			}
			if account.Reverse.Sniffing != nil { // may not be reached: error json unmarshal
				return nil, errors.New(`VLESS clients: inbound's "reverse" can't have "sniffing"`)
			}
		}

		user.Account = serial.ToTypedMessage(account)
		config.Clients[idx] = user
	}

	config.Decryption = c.Decryption
	if !func() bool {
		s := strings.Split(config.Decryption, ".")
		if len(s) < 4 || s[0] != "mlkem768x25519plus" {
			return false
		}
		switch s[1] {
		case "native":
		case "xorpub":
			config.XorMode = 1
		case "random":
			config.XorMode = 2
		default:
			return false
		}
		t := strings.SplitN(strings.TrimSuffix(s[2], "s"), "-", 2)
		i, err := strconv.Atoi(t[0])
		if err != nil {
			return false
		}
		config.SecondsFrom = int64(i)
		if len(t) == 2 {
			i, err := strconv.Atoi(t[1])
			if err != nil {
				return false
			}
			config.SecondsTo = int64(i)
		}
		padding := 0
		for _, r := range s[3:] {
			if len(r) < 20 {
				padding += len(r) + 1
				continue
			}
			if b, _ := base64.RawURLEncoding.DecodeString(r); len(b) != 32 && len(b) != 64 {
				return false
			}
		}
		config.Decryption = config.Decryption[27+len(s[2]):]
		if padding > 0 {
			config.Padding = config.Decryption[:padding-1]
			config.Decryption = config.Decryption[padding:]
		}
		return true
	}() && config.Decryption != "none" {
		if config.Decryption == "" {
			return nil, errors.New(`VLESS settings: please add/set "decryption":"none" to every settings`)
		}
		return nil, errors.New(`VLESS settings: unsupported "decryption": ` + config.Decryption)
	}

	if config.Decryption != "none" && c.Fallbacks != nil {
		return nil, errors.New(`VLESS settings: "fallbacks" can not be used together with "decryption"`)
	}

	for _, fb := range c.Fallbacks {
		var i uint16
		var s string
		if err := json.Unmarshal(fb.Dest, &i); err == nil {
			s = strconv.Itoa(int(i))
		} else {
			_ = json.Unmarshal(fb.Dest, &s)
		}
		config.Fallbacks = append(config.Fallbacks, &inbound.Fallback{
			Name: fb.Name,
			Alpn: fb.Alpn,
			Path: fb.Path,
			Type: fb.Type,
			Dest: s,
			Xver: fb.Xver,
		})
	}
	for _, fb := range config.Fallbacks {
		/*
			if fb.Alpn == "h2" && fb.Path != "" {
				return nil, errors.New(`VLESS fallbacks: "alpn":"h2" doesn't support "path"`)
			}
		*/
		if fb.Path != "" && fb.Path[0] != '/' {
			return nil, errors.New(`VLESS fallbacks: "path" must be empty or start with "/"`)
		}
		if fb.Type == "" && fb.Dest != "" {
			if fb.Dest == "serve-ws-none" {
				fb.Type = "serve"
			} else if filepath.IsAbs(fb.Dest) || fb.Dest[0] == '@' {
				fb.Type = "unix"
				if strings.HasPrefix(fb.Dest, "@@") && (runtime.GOOS == "linux" || runtime.GOOS == "android") {
					fullAddr := make([]byte, len(syscall.RawSockaddrUnix{}.Path)) // may need padding to work with haproxy
					copy(fullAddr, fb.Dest[1:])
					fb.Dest = string(fullAddr)
				}
			} else {
				if _, err := strconv.Atoi(fb.Dest); err == nil {
					fb.Dest = "localhost:" + fb.Dest
				}
				if _, _, err := net.SplitHostPort(fb.Dest); err == nil {
					fb.Type = "tcp"
				}
			}
		}
		if fb.Type == "" {
			return nil, errors.New(`VLESS fallbacks: please fill in a valid value for every "dest"`)
		}
		if fb.Xver > 2 {
			return nil, errors.New(`VLESS fallbacks: invalid PROXY protocol version, "xver" only accepts 0, 1, 2`)
		}
	}

	return config, nil
}

type VLessReverseConfig struct {
	Tag      string          `json:"tag"`
	Sniffing *SniffingConfig `json:"sniffing"`
}

func (c *VLessReverseConfig) Build() (*vless.Reverse, error) {
	if c.Tag == "" {
		return nil, errors.New(`VLESS reverse: "tag" can't be empty`)
	}
	r := &vless.Reverse{
		Tag: c.Tag,
	}
	if c.Sniffing != nil {
		sc, err := c.Sniffing.Build()
		if err != nil {
			return nil, errors.New(`VLESS reverse: invalid "sniffing" config`).Base(err)
		}
		r.Sniffing = sc
	}
	return r, nil
}

type VLessOutboundVnext struct {
	Address *Address          `json:"address"`
	Port    uint16            `json:"port"`
	Users   []json.RawMessage `json:"users"`
}

type VLessOutboundConfig struct {
	Address    *Address              `json:"address"`
	Port       uint16                `json:"port"`
	Level      uint32                `json:"level"`
	Email      string                `json:"email"`
	Id         string                `json:"id"`
	Flow       string                `json:"flow"`
	Seed       string                `json:"seed"`
	Encryption string                `json:"encryption"`
	Reverse    *VLessReverseConfig   `json:"reverse"`
	Testpre    uint32                `json:"testpre"`
	Testseed   []uint32              `json:"testseed"`
	Vnext      []*VLessOutboundVnext `json:"vnext"`
}

// Build implements Buildable
func (c *VLessOutboundConfig) Build() (proto.Message, error) {
	config := new(outbound.Config)
	if c.Address != nil {
		c.Vnext = []*VLessOutboundVnext{
			{
				Address: c.Address,
				Port:    c.Port,
				Users:   []json.RawMessage{{}},
			},
		}
	}
	if len(c.Vnext) != 1 {
		return nil, errors.New(`VLESS settings: "vnext" should have one and only one member. Multiple endpoints in "vnext" should use multiple VLESS outbounds and routing balancer instead`)
	}
	for _, rec := range c.Vnext {
		if rec.Address == nil {
			return nil, errors.New(`VLESS vnext: "address" is not set`)
		}
		if len(rec.Users) != 1 {
			return nil, errors.New(`VLESS vnext: "users" should have one and only one member. Multiple members in "users" should use multiple VLESS outbounds and routing balancer instead`)
		}
		spec := &protocol.ServerEndpoint{
			Address: rec.Address.Build(),
			Port:    uint32(rec.Port),
		}
		for _, rawUser := range rec.Users {
			user := new(protocol.User)
			if c.Address != nil {
				user.Level = c.Level
				user.Email = c.Email
			} else {
				if err := json.Unmarshal(rawUser, user); err != nil {
					return nil, errors.New(`VLESS users: invalid user`).Base(err)
				}
			}
			account := new(vless.Account)
			if c.Address != nil {
				account.Id = c.Id
				account.Flow = c.Flow
				//account.Seed = c.Seed
				account.Encryption = c.Encryption
				if c.Reverse != nil {
					rvs, err := c.Reverse.Build()
					if err != nil {
						return nil, err
					}
					account.Reverse = rvs
				}
				account.Testpre = c.Testpre
				account.Testseed = c.Testseed
			} else {
				if err := json.Unmarshal(rawUser, account); err != nil {
					return nil, errors.New(`VLESS users: invalid user`).Base(err)
				}
				if account.Reverse != nil { // may not be reached: error json unmarshal
					return nil, errors.New(`VLESS users: please use simplified outbound's config style to use "reverse"`)
				}
			}

			u, err := uuid.ParseString(account.Id)
			if err != nil {
				return nil, err
			}
			account.Id = u.String()

			switch account.Flow {
			case "":
			case vless.XRV, vless.XRV + "-udp443":
			default:
				return nil, errors.New(`VLESS users: "flow" doesn't support "` + account.Flow + `" in this version`)
			}

			if !func() bool {
				s := strings.Split(account.Encryption, ".")
				if len(s) < 4 || s[0] != "mlkem768x25519plus" {
					return false
				}
				switch s[1] {
				case "native":
				case "xorpub":
					account.XorMode = 1
				case "random":
					account.XorMode = 2
				default:
					return false
				}
				switch s[2] {
				case "1rtt":
				case "0rtt":
					account.Seconds = 1
				default:
					return false
				}
				padding := 0
				for _, r := range s[3:] {
					if len(r) < 20 {
						padding += len(r) + 1
						continue
					}
					if b, _ := base64.RawURLEncoding.DecodeString(r); len(b) != 32 && len(b) != 1184 {
						return false
					}
				}
				account.Encryption = account.Encryption[27+len(s[2]):]
				if padding > 0 {
					account.Padding = account.Encryption[:padding-1]
					account.Encryption = account.Encryption[padding:]
				}
				return true
			}() && account.Encryption != "none" {
				if account.Encryption == "" {
					return nil, errors.New(`VLESS users: please add/set "encryption":"none" for every user`)
				}
				return nil, errors.New(`VLESS users: unsupported "encryption": ` + account.Encryption)
			}

			user.Account = serial.ToTypedMessage(account)
			spec.User = user
			break
		}
		config.Vnext = spec
		break
	}

	return config, nil
}
