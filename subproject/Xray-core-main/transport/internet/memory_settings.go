package internet

import (
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet/finalmask"
)

// MemoryStreamConfig is a parsed form of StreamConfig. It is used to reduce the number of Protobuf parses.
type MemoryStreamConfig struct {
	Destination      *net.Destination
	ProtocolName     string
	ProtocolSettings interface{}
	SecurityType     string
	SecuritySettings interface{}
	TcpmaskManager   *finalmask.TcpmaskManager
	UdpmaskManager   *finalmask.UdpmaskManager
	QuicParams       *QuicParams
	SocketSettings   *SocketConfig
	DownloadSettings *MemoryStreamConfig
}

// ToMemoryStreamConfig converts a StreamConfig to MemoryStreamConfig. It returns a default non-nil MemoryStreamConfig for nil input.
func ToMemoryStreamConfig(s *StreamConfig) (*MemoryStreamConfig, error) {
	ets, err := s.GetEffectiveTransportSettings()
	if err != nil {
		return nil, err
	}

	mss := &MemoryStreamConfig{
		ProtocolName:     s.GetEffectiveProtocol(),
		ProtocolSettings: ets,
	}

	if s != nil {
		if s.Address != nil {
			mss.Destination = &net.Destination{
				Address: s.Address.AsAddress(),
				Port:    net.Port(s.Port),
				Network: net.Network_TCP,
			}
		}
		mss.SocketSettings = s.SocketSettings
	}

	if s != nil && s.HasSecuritySettings() {
		ess, err := s.GetEffectiveSecuritySettings()
		if err != nil {
			return nil, err
		}
		mss.SecurityType = s.SecurityType
		mss.SecuritySettings = ess
	}

	if s != nil && len(s.Tcpmasks) > 0 {
		var masks []finalmask.Tcpmask
		for _, msg := range s.Tcpmasks {
			instance, err := msg.GetInstance()
			if err != nil {
				return nil, err
			}
			masks = append(masks, instance.(finalmask.Tcpmask))
		}
		mss.TcpmaskManager = finalmask.NewTcpmaskManager(masks)
	}

	if s != nil && s.QuicParams != nil {
		mss.QuicParams = s.QuicParams
	}

	if s != nil && len(s.Udpmasks) > 0 {
		var masks []finalmask.Udpmask
		for _, msg := range s.Udpmasks {
			instance, err := msg.GetInstance()
			if err != nil {
				return nil, err
			}
			masks = append(masks, instance.(finalmask.Udpmask))
		}
		mss.UdpmaskManager = finalmask.NewUdpmaskManager(masks)
	}

	return mss, nil
}
