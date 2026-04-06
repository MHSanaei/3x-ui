package hysteria

import (
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/hysteria/padding"
)

const (
	closeErrCodeOK            = 0x100 // HTTP3 ErrCodeNoError
	closeErrCodeProtocolError = 0x101 // HTTP3 ErrCodeGeneralProtocolError

	MaxDatagramFrameSize = 1200

	URLHost = "hysteria"
	URLPath = "/auth"

	RequestHeaderAuth        = "Hysteria-Auth"
	ResponseHeaderUDPEnabled = "Hysteria-UDP"
	CommonHeaderCCRX         = "Hysteria-CC-RX"
	CommonHeaderPadding      = "Hysteria-Padding"

	StatusAuthOK = 233

	udpMessageChanSize = 1024

	FrameTypeTCPRequest = 0x401

	idleCleanupInterval = 1 * time.Second
)

var (
	authRequestPadding  = padding.Padding{Min: 256, Max: 2048}
	authResponsePadding = padding.Padding{Min: 256, Max: 2048}
)

type Status int

const (
	StatusUnknown Status = iota
	StatusActive
	StatusInactive
)

const protocolName = "hysteria"

func init() {
	common.Must(internet.RegisterProtocolConfigCreator(protocolName, func() interface{} {
		return new(Config)
	}))
}
