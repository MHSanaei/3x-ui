package dns

import (
	"context"
	go_errors "errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	dns_proto "github.com/xtls/xray-core/common/protocol/dns"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/common/signal"
	"github.com/xtls/xray-core/common/task"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/dns"
	"github.com/xtls/xray-core/features/policy"
	"github.com/xtls/xray-core/transport"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
	"golang.org/x/net/dns/dnsmessage"
)

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		h := new(Handler)
		if err := core.RequireFeatures(ctx, func(dnsClient dns.Client, policyManager policy.Manager) error {
			core.OptionalFeatures(ctx, func(fdns dns.FakeDNSEngine) {
				h.fdns = fdns
			})
			return h.Init(config.(*Config), dnsClient, policyManager)
		}); err != nil {
			return nil, err
		}
		return h, nil
	}))
}

type ownLinkVerifier interface {
	IsOwnLink(ctx context.Context) bool
}

type Handler struct {
	client          dns.Client
	fdns            dns.FakeDNSEngine
	ownLinkVerifier ownLinkVerifier
	server          net.Destination
	timeout         time.Duration
	nonIPQuery      string
	blockTypes      []int32
}

func (h *Handler) Init(config *Config, dnsClient dns.Client, policyManager policy.Manager) error {
	h.client = dnsClient
	h.timeout = policyManager.ForLevel(config.UserLevel).Timeouts.ConnectionIdle

	if v, ok := dnsClient.(ownLinkVerifier); ok {
		h.ownLinkVerifier = v
	}

	if config.Server != nil {
		h.server = config.Server.AsDestination()
	}
	h.nonIPQuery = config.Non_IPQuery
	if h.nonIPQuery == "" {
		h.nonIPQuery = "reject"
	}
	h.blockTypes = config.BlockTypes
	return nil
}

func (h *Handler) isOwnLink(ctx context.Context) bool {
	return h.ownLinkVerifier != nil && h.ownLinkVerifier.IsOwnLink(ctx)
}

func parseIPQuery(b []byte) (r bool, domain string, id uint16, qType dnsmessage.Type) {
	var parser dnsmessage.Parser
	header, err := parser.Start(b)
	if err != nil {
		errors.LogInfoInner(context.Background(), err, "parser start")
		return
	}

	id = header.ID
	q, err := parser.Question()
	if err != nil {
		errors.LogInfoInner(context.Background(), err, "question")
		return
	}
	domain = q.Name.String()
	qType = q.Type
	if qType != dnsmessage.TypeA && qType != dnsmessage.TypeAAAA {
		return
	}

	r = true
	return
}

// Process implements proxy.Outbound.
func (h *Handler) Process(ctx context.Context, link *transport.Link, d internet.Dialer) error {
	outbounds := session.OutboundsFromContext(ctx)
	ob := outbounds[len(outbounds)-1]
	if !ob.Target.IsValid() {
		return errors.New("invalid outbound")
	}
	ob.Name = "dns"

	srcNetwork := ob.Target.Network

	dest := ob.Target
	if h.server.Network != net.Network_Unknown {
		dest.Network = h.server.Network
	}
	if h.server.Address != nil {
		dest.Address = h.server.Address
	}
	if h.server.Port != 0 {
		dest.Port = h.server.Port
	}

	errors.LogInfo(ctx, "handling DNS traffic to ", dest)

	conn := &outboundConn{
		dialer: func() (stat.Connection, error) {
			return d.Dial(ctx, dest)
		},
		connReady: make(chan struct{}, 1),
	}

	var reader dns_proto.MessageReader
	var writer dns_proto.MessageWriter
	if srcNetwork == net.Network_TCP {
		reader = dns_proto.NewTCPReader(link.Reader)
		writer = &dns_proto.TCPWriter{
			Writer: link.Writer,
		}
	} else {
		reader = &dns_proto.UDPReader{
			Reader: link.Reader,
		}
		writer = &dns_proto.UDPWriter{
			Writer: link.Writer,
		}
	}

	var connReader dns_proto.MessageReader
	var connWriter dns_proto.MessageWriter
	if dest.Network == net.Network_TCP {
		connReader = dns_proto.NewTCPReader(buf.NewReader(conn))
		connWriter = &dns_proto.TCPWriter{
			Writer: buf.NewWriter(conn),
		}
	} else {
		connReader = &dns_proto.UDPReader{
			Reader: buf.NewPacketReader(conn),
		}
		connWriter = &dns_proto.UDPWriter{
			Writer: buf.NewWriter(conn),
		}
	}

	if session.TimeoutOnlyFromContext(ctx) {
		ctx, _ = context.WithCancel(context.Background())
	}

	ctx, cancel := context.WithCancel(ctx)
	terminate := func() {
		cancel()
		conn.Close()
	}
	timer := signal.CancelAfterInactivity(ctx, terminate, h.timeout)
	defer timer.SetTimeout(0)

	request := func() error {
		defer timer.SetTimeout(0)
		for {
			b, err := reader.ReadMessage()
			if err == io.EOF {
				return nil
			}

			if err != nil {
				return err
			}

			timer.Update()

			if !h.isOwnLink(ctx) {
				isIPQuery, domain, id, qType := parseIPQuery(b.Bytes())
				if len(h.blockTypes) > 0 {
					for _, blocktype := range h.blockTypes {
						if blocktype == int32(qType) {
							b.Release()
							errors.LogInfo(ctx, "blocked type ", qType, " query for domain ", domain)
							if h.nonIPQuery == "reject" {
								err := h.rejectNonIPQuery(id, qType, domain, writer)
								if err != nil {
									return err
								}
							}
							return nil
						}
					}
				}
				if isIPQuery {
					b.Release()
					go h.handleIPQuery(id, qType, domain, writer, timer)
					continue
				}
				if h.nonIPQuery == "drop" {
					b.Release()
					continue
				}
				if h.nonIPQuery == "reject" {
					b.Release()
					err := h.rejectNonIPQuery(id, qType, domain, writer)
					if err != nil {
						return err
					}
					continue
				}
			}

			if err := connWriter.WriteMessage(b); err != nil {
				return err
			}
		}
	}

	response := func() error {
		defer timer.SetTimeout(0)
		for {
			b, err := connReader.ReadMessage()
			if err == io.EOF {
				return nil
			}

			if err != nil {
				return err
			}

			timer.Update()

			if err := writer.WriteMessage(b); err != nil {
				return err
			}
		}
	}

	if err := task.Run(ctx, request, response); err != nil {
		return errors.New("connection ends").Base(err)
	}

	return nil
}

func (h *Handler) handleIPQuery(id uint16, qType dnsmessage.Type, domain string, writer dns_proto.MessageWriter, timer *signal.ActivityTimer) {
	var ips []net.IP
	var err error

	var ttl4 uint32
	var ttl6 uint32

	switch qType {
	case dnsmessage.TypeA:
		ips, ttl4, err = h.client.LookupIP(domain, dns.IPOption{
			IPv4Enable: true,
			IPv6Enable: false,
			FakeEnable: true,
		})
	case dnsmessage.TypeAAAA:
		ips, ttl6, err = h.client.LookupIP(domain, dns.IPOption{
			IPv4Enable: false,
			IPv6Enable: true,
			FakeEnable: true,
		})
	}

	rcode := dns.RCodeFromError(err)
	if rcode == 0 && len(ips) == 0 && !go_errors.Is(err, dns.ErrEmptyResponse) {
		errors.LogInfoInner(context.Background(), err, "ip query")
		return
	}

	switch qType {
	case dnsmessage.TypeA:
		for i, ip := range ips {
			ips[i] = ip.To4()
		}
	case dnsmessage.TypeAAAA:
		for i, ip := range ips {
			ips[i] = ip.To16()
		}
	}

	b := buf.New()
	rawBytes := b.Extend(buf.Size)
	builder := dnsmessage.NewBuilder(rawBytes[:0], dnsmessage.Header{
		ID:                 id,
		RCode:              dnsmessage.RCode(rcode),
		RecursionAvailable: true,
		RecursionDesired:   true,
		Response:           true,
		Authoritative:      true,
	})
	builder.EnableCompression()
	common.Must(builder.StartQuestions())
	common.Must(builder.Question(dnsmessage.Question{
		Name:  dnsmessage.MustNewName(domain),
		Class: dnsmessage.ClassINET,
		Type:  qType,
	}))
	common.Must(builder.StartAnswers())

	rHeader4 := dnsmessage.ResourceHeader{Name: dnsmessage.MustNewName(domain), Class: dnsmessage.ClassINET, TTL: ttl4}
	rHeader6 := dnsmessage.ResourceHeader{Name: dnsmessage.MustNewName(domain), Class: dnsmessage.ClassINET, TTL: ttl6}
	for _, ip := range ips {
		if len(ip) == net.IPv4len {
			var r dnsmessage.AResource
			copy(r.A[:], ip)
			common.Must(builder.AResource(rHeader4, r))
		} else {
			var r dnsmessage.AAAAResource
			copy(r.AAAA[:], ip)
			common.Must(builder.AAAAResource(rHeader6, r))
		}
	}
	msgBytes, err := builder.Finish()
	if err != nil {
		errors.LogInfoInner(context.Background(), err, "pack message")
		b.Release()
		timer.SetTimeout(0)
	}
	b.Resize(0, int32(len(msgBytes)))

	if err := writer.WriteMessage(b); err != nil {
		errors.LogInfoInner(context.Background(), err, "write IP answer")
		timer.SetTimeout(0)
	}
}

func (h *Handler) rejectNonIPQuery(id uint16, qType dnsmessage.Type, domain string, writer dns_proto.MessageWriter) error {
	domainT := strings.TrimSuffix(domain, ".")
	if domainT == "" {
		return errors.New("empty domain name")
	}
	b := buf.New()
	rawBytes := b.Extend(buf.Size)
	builder := dnsmessage.NewBuilder(rawBytes[:0], dnsmessage.Header{
		ID:                 id,
		RCode:              dnsmessage.RCodeRefused,
		RecursionAvailable: true,
		RecursionDesired:   true,
		Response:           true,
		Authoritative:      true,
	})
	builder.EnableCompression()
	common.Must(builder.StartQuestions())
	err := builder.Question(dnsmessage.Question{
		Name:  dnsmessage.MustNewName(domain),
		Class: dnsmessage.ClassINET,
		Type:  qType,
	})
	if err != nil {
		errors.LogInfo(context.Background(), "unexpected domain ", domain, " when building reject message: ", err)
		b.Release()
		return err
	}

	msgBytes, err := builder.Finish()
	if err != nil {
		errors.LogInfoInner(context.Background(), err, "pack reject message")
		b.Release()
		return err
	}
	b.Resize(0, int32(len(msgBytes)))

	if err := writer.WriteMessage(b); err != nil {
		errors.LogInfoInner(context.Background(), err, "write reject answer")
		return err
	}
	return nil
}

type outboundConn struct {
	access sync.Mutex
	dialer func() (stat.Connection, error)

	conn      net.Conn
	connReady chan struct{}
	closed    bool
}

func (c *outboundConn) dial() error {
	conn, err := c.dialer()
	if err != nil {
		return err
	}
	c.conn = conn
	c.connReady <- struct{}{}
	return nil
}

func (c *outboundConn) Write(b []byte) (int, error) {
	c.access.Lock()
	if c.closed {
		c.access.Unlock()
		return 0, errors.New("outbound connection closed")
	}

	if c.conn == nil {
		if err := c.dial(); err != nil {
			c.access.Unlock()
			errors.LogWarningInner(context.Background(), err, "failed to dial outbound connection")
			return 0, err
		}
	}

	c.access.Unlock()

	return c.conn.Write(b)
}

func (c *outboundConn) Read(b []byte) (int, error) {
	c.access.Lock()
	if c.closed {
		c.access.Unlock()
		return 0, io.EOF
	}

	if c.conn == nil {
		c.access.Unlock()
		_, open := <-c.connReady
		if !open {
			return 0, io.EOF
		}
		return c.conn.Read(b)
	}
	c.access.Unlock()
	return c.conn.Read(b)
}

func (c *outboundConn) Close() error {
	c.access.Lock()
	c.closed = true
	close(c.connReady)
	if c.conn != nil {
		c.conn.Close()
	}
	c.access.Unlock()
	return nil
}
