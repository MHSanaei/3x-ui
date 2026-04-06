package log

import (
	"context"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/log"
)

// Instance is a log.Handler that handles logs.
type Instance struct {
	sync.RWMutex
	config       *Config
	accessLogger log.Handler
	errorLogger  log.Handler
	active       bool
	dns          bool
	mask4        int
	mask6        int
}

// New creates a new log.Instance based on the given config.
func New(ctx context.Context, config *Config) (*Instance, error) {
	m4, m6, err := ParseMaskAddress(config.MaskAddress)
	if err != nil {
		return nil, err
	}

	g := &Instance{
		config: config,
		active: false,
		dns:    config.EnableDnsLog,
		mask4:  m4,
		mask6:  m6,
	}
	log.RegisterHandler(g)

	// start logger now,
	// then other modules will be able to log during initialization
	if err := g.startInternal(); err != nil {
		return nil, err
	}

	errors.LogDebug(ctx, "Logger started")
	return g, nil
}

func (g *Instance) initAccessLogger() error {
	handler, err := createHandler(g.config.AccessLogType, HandlerCreatorOptions{
		Path: g.config.AccessLogPath,
	})
	if err != nil {
		return err
	}
	g.accessLogger = handler
	return nil
}

func (g *Instance) initErrorLogger() error {
	handler, err := createHandler(g.config.ErrorLogType, HandlerCreatorOptions{
		Path: g.config.ErrorLogPath,
	})
	if err != nil {
		return err
	}
	g.errorLogger = handler
	return nil
}

// Type implements common.HasType.
func (*Instance) Type() interface{} {
	return (*Instance)(nil)
}

func (g *Instance) startInternal() error {
	g.Lock()
	defer g.Unlock()

	if g.active {
		return nil
	}

	g.active = true

	if err := g.initAccessLogger(); err != nil {
		return errors.New("failed to initialize access logger").Base(err).AtWarning()
	}
	if err := g.initErrorLogger(); err != nil {
		return errors.New("failed to initialize error logger").Base(err).AtWarning()
	}

	return nil
}

// Start implements common.Runnable.Start().
func (g *Instance) Start() error {
	return g.startInternal()
}

// Handle implements log.Handler.
func (g *Instance) Handle(msg log.Message) {
	g.RLock()
	defer g.RUnlock()

	if !g.active {
		return
	}

	var Msg log.Message
	if g.config.MaskAddress != "" {
		Msg = &MaskedMsgWrapper{
			Message: msg,
			Mask4:   g.mask4,
			Mask6:   g.mask6,
		}
	} else {
		Msg = msg
	}

	switch msg := msg.(type) {
	case *log.AccessMessage:
		if g.accessLogger != nil {
			g.accessLogger.Handle(Msg)
		}
	case *log.DNSLog:
		if g.dns && g.accessLogger != nil {
			g.accessLogger.Handle(Msg)
		}
	case *log.GeneralMessage:
		if g.errorLogger != nil && msg.Severity <= g.config.ErrorLogLevel {
			g.errorLogger.Handle(Msg)
		}
	default:
		// Swallow
	}
}

// Close implements common.Closable.Close().
func (g *Instance) Close() error {
	errors.LogDebug(context.Background(), "Logger closing")

	g.Lock()
	defer g.Unlock()

	if !g.active {
		return nil
	}

	g.active = false

	common.Close(g.accessLogger)
	g.accessLogger = nil

	common.Close(g.errorLogger)
	g.errorLogger = nil

	return nil
}

func ParseMaskAddress(c string) (int, int, error) {
	var m4, m6 int
	switch c {
	case "half":
		m4, m6 = 16, 32
	case "quarter":
		m4, m6 = 8, 16
	case "full":
		m4, m6 = 0, 0
	case "":
		// do nothing
	default:
		if parts := strings.Split(c, "+"); len(parts) > 0 {
			if len(parts) >= 1 && parts[0] != "" {
				i, err := strconv.Atoi(strings.TrimPrefix(parts[0], "/"))
				if err != nil {
					return 32, 128, err
				}
				m4 = i
			}
			if len(parts) >= 2 && parts[1] != "" {
				i, err := strconv.Atoi(strings.TrimPrefix(parts[1], "/"))
				if err != nil {
					return 32, 128, err
				}
				m6 = i
			}
		}
	}

	if m4%8 != 0 || m4 > 32 || m4 < 0 {
		return 32, 128, errors.New("Log Mask: ipv4 mask must be divisible by 8 and between 0-32")
	}

	return m4, m6, nil
}

// MaskedMsgWrapper is to wrap the string() method to mask IP addresses in the log.
type MaskedMsgWrapper struct {
	log.Message
	Mask4 int
	Mask6 int
}

var (
	ipv4Regex = regexp.MustCompile(`(\d{1,3}\.){3}\d{1,3}`)
	ipv6Regex = regexp.MustCompile(`(?:[\da-fA-F]{0,4}:[\da-fA-F]{0,4}){2,7}`)
)

func (m *MaskedMsgWrapper) String() string {
	str := m.Message.String()

	// Process ipv4
	maskedMsg := ipv4Regex.ReplaceAllStringFunc(str, func(s string) string {
		if m.Mask4 == 32 {
			return s
		}
		if m.Mask4 == 0 {
			return "[Masked IPv4]"
		}

		parts := strings.Split(s, ".")
		for i := m.Mask4 / 8; i < 4; i++ {
			parts[i] = "*"
		}
		return strings.Join(parts, ".")
	})

	// process ipv6
	maskedMsg = ipv6Regex.ReplaceAllStringFunc(maskedMsg, func(s string) string {
		if m.Mask6 == 128 {
			return s
		}
		if m.Mask6 == 0 {
			return "Masked IPv6"
		}
		ip := net.ParseIP(s)
		if ip == nil {
			return s
		}
		return ip.Mask(net.CIDRMask(m.Mask6, 128)).String() + "/" + strconv.Itoa(m.Mask6)
	})

	return maskedMsg
}

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return New(ctx, config.(*Config))
	}))
}
