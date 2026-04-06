package conf

import (
	"encoding/base64"
	"encoding/hex"
	"net"
	"strings"

	"github.com/xtls/xray-core/common/errors"
	v2net "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/proxy/freedom"
	"github.com/xtls/xray-core/transport/internet"
	"google.golang.org/protobuf/proto"
)

type FreedomConfig struct {
	TargetStrategy string    `json:"targetStrategy"`
	DomainStrategy string    `json:"domainStrategy"`
	Redirect       string    `json:"redirect"`
	UserLevel      uint32    `json:"userLevel"`
	Fragment       *Fragment `json:"fragment"`
	Noise          *Noise    `json:"noise"`
	Noises         []*Noise  `json:"noises"`
	ProxyProtocol  uint32    `json:"proxyProtocol"`
}

type Fragment struct {
	Packets  string      `json:"packets"`
	Length   *Int32Range `json:"length"`
	Interval *Int32Range `json:"interval"`
	MaxSplit *Int32Range `json:"maxSplit"`
}

type Noise struct {
	Type    string      `json:"type"`
	Packet  string      `json:"packet"`
	Delay   *Int32Range `json:"delay"`
	ApplyTo string      `json:"applyTo"`
}

// Build implements Buildable
func (c *FreedomConfig) Build() (proto.Message, error) {
	config := new(freedom.Config)
	targetStrategy := c.TargetStrategy
	if targetStrategy == "" {
		targetStrategy = c.DomainStrategy
	}
	switch strings.ToLower(targetStrategy) {
	case "asis", "":
		config.DomainStrategy = internet.DomainStrategy_AS_IS
	case "useip":
		config.DomainStrategy = internet.DomainStrategy_USE_IP
	case "useipv4":
		config.DomainStrategy = internet.DomainStrategy_USE_IP4
	case "useipv6":
		config.DomainStrategy = internet.DomainStrategy_USE_IP6
	case "useipv4v6":
		config.DomainStrategy = internet.DomainStrategy_USE_IP46
	case "useipv6v4":
		config.DomainStrategy = internet.DomainStrategy_USE_IP64
	case "forceip":
		config.DomainStrategy = internet.DomainStrategy_FORCE_IP
	case "forceipv4":
		config.DomainStrategy = internet.DomainStrategy_FORCE_IP4
	case "forceipv6":
		config.DomainStrategy = internet.DomainStrategy_FORCE_IP6
	case "forceipv4v6":
		config.DomainStrategy = internet.DomainStrategy_FORCE_IP46
	case "forceipv6v4":
		config.DomainStrategy = internet.DomainStrategy_FORCE_IP64
	default:
		return nil, errors.New("unsupported domain strategy: ", targetStrategy)
	}

	if c.Fragment != nil {
		config.Fragment = new(freedom.Fragment)

		switch strings.ToLower(c.Fragment.Packets) {
		case "tlshello":
			// TLS Hello Fragmentation (into multiple handshake messages)
			config.Fragment.PacketsFrom = 0
			config.Fragment.PacketsTo = 1
		case "":
			// TCP Segmentation (all packets)
			config.Fragment.PacketsFrom = 0
			config.Fragment.PacketsTo = 0
		default:
			// TCP Segmentation (range)
			from, to, err := ParseRangeString(c.Fragment.Packets)
			if err != nil {
				return nil, errors.New("Invalid PacketsFrom").Base(err)
			}
			config.Fragment.PacketsFrom = uint64(from)
			config.Fragment.PacketsTo = uint64(to)
			if config.Fragment.PacketsFrom == 0 {
				return nil, errors.New("PacketsFrom can't be 0")
			}
		}

		{
			if c.Fragment.Length == nil {
				return nil, errors.New("Length can't be empty")
			}
			config.Fragment.LengthMin = uint64(c.Fragment.Length.From)
			config.Fragment.LengthMax = uint64(c.Fragment.Length.To)
			if config.Fragment.LengthMin == 0 {
				return nil, errors.New("LengthMin can't be 0")
			}
		}

		{
			if c.Fragment.Interval == nil {
				return nil, errors.New("Interval can't be empty")
			}
			config.Fragment.IntervalMin = uint64(c.Fragment.Interval.From)
			config.Fragment.IntervalMax = uint64(c.Fragment.Interval.To)
		}

		{
			if c.Fragment.MaxSplit != nil {
				config.Fragment.MaxSplitMin = uint64(c.Fragment.MaxSplit.From)
				config.Fragment.MaxSplitMax = uint64(c.Fragment.MaxSplit.To)
			}
		}
	}

	if c.Noise != nil {
		return nil, errors.PrintRemovedFeatureError("noise = { ... }", "noises = [ { ... } ]")
	}

	if c.Noises != nil {
		for _, n := range c.Noises {
			NConfig, err := ParseNoise(n)
			if err != nil {
				return nil, err
			}
			config.Noises = append(config.Noises, NConfig)
		}
	}

	config.UserLevel = c.UserLevel
	if len(c.Redirect) > 0 {
		host, portStr, err := net.SplitHostPort(c.Redirect)
		if err != nil {
			return nil, errors.New("invalid redirect address: ", c.Redirect, ": ", err).Base(err)
		}
		port, err := v2net.PortFromString(portStr)
		if err != nil {
			return nil, errors.New("invalid redirect port: ", c.Redirect, ": ", err).Base(err)
		}
		config.DestinationOverride = &freedom.DestinationOverride{
			Server: &protocol.ServerEndpoint{
				Port: uint32(port),
			},
		}

		if len(host) > 0 {
			config.DestinationOverride.Server.Address = v2net.NewIPOrDomain(v2net.ParseAddress(host))
		}
	}
	if c.ProxyProtocol > 0 && c.ProxyProtocol <= 2 {
		config.ProxyProtocol = c.ProxyProtocol
	}
	return config, nil
}

func ParseNoise(noise *Noise) (*freedom.Noise, error) {
	var err error
	NConfig := new(freedom.Noise)
	noise.Packet = strings.TrimSpace(noise.Packet)

	switch noise.Type {
	case "rand":
		min, max, err := ParseRangeString(noise.Packet)
		if err != nil {
			return nil, errors.New("invalid value for rand Length").Base(err)
		}
		NConfig.LengthMin = uint64(min)
		NConfig.LengthMax = uint64(max)
		if NConfig.LengthMin == 0 {
			return nil, errors.New("rand lengthMin or lengthMax cannot be 0")
		}

	case "str":
		// user input string
		NConfig.Packet = []byte(noise.Packet)

	case "hex":
		// user input hex
		NConfig.Packet, err = hex.DecodeString(noise.Packet)
		if err != nil {
			return nil, errors.New("Invalid hex string").Base(err)
		}

	case "base64":
		// user input base64
		NConfig.Packet, err = base64.RawURLEncoding.DecodeString(strings.NewReplacer("+", "-", "/", "_", "=", "").Replace(noise.Packet))
		if err != nil {
			return nil, errors.New("Invalid base64 string").Base(err)
		}

	default:
		return nil, errors.New("Invalid packet, only rand/str/hex/base64 are supported")
	}

	if noise.Delay != nil {
		NConfig.DelayMin = uint64(noise.Delay.From)
		NConfig.DelayMax = uint64(noise.Delay.To)
	}
	switch strings.ToLower(noise.ApplyTo) {
	case "", "ip", "all":
		NConfig.ApplyTo = "ip"
	case "ipv4":
		NConfig.ApplyTo = "ipv4"
	case "ipv6":
		NConfig.ApplyTo = "ipv6"
	default:
		return nil, errors.New("Invalid applyTo, only ip/ipv4/ipv6 are supported")
	}
	return NConfig, nil
}
