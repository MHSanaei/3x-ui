package dns

import (
	"context"
	"encoding/binary"
	"math"
	"strings"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/core"
	dns_feature "github.com/xtls/xray-core/features/dns"

	"golang.org/x/net/dns/dnsmessage"
)

// Fqdn normalizes domain make sure it ends with '.'
// case-sensitive
func Fqdn(domain string) string {
	if len(domain) > 0 && strings.HasSuffix(domain, ".") {
		return domain
	}
	return domain + "."
}

type record struct {
	A    *IPRecord
	AAAA *IPRecord
}

// IPRecord is a cacheable item for a resolved domain
type IPRecord struct {
	ReqID     uint16
	IP        []net.IP
	Expire    time.Time
	RCode     dnsmessage.RCode
	RawHeader *dnsmessage.Header
}

func (r *IPRecord) getIPs() ([]net.IP, int32, error) {
	if r == nil {
		return nil, 0, errRecordNotFound
	}

	untilExpire := time.Until(r.Expire).Seconds()
	ttl := int32(math.Ceil(untilExpire))

	if r.RCode != dnsmessage.RCodeSuccess {
		return nil, ttl, dns_feature.RCodeError(r.RCode)
	}
	if len(r.IP) == 0 {
		return nil, ttl, dns_feature.ErrEmptyResponse
	}

	return r.IP, ttl, nil
}

var errRecordNotFound = errors.New("record not found")

type dnsRequest struct {
	reqType dnsmessage.Type
	domain  string
	start   time.Time
	expire  time.Time
	msg     *dnsmessage.Message
}

func genEDNS0Options(clientIP net.IP, padding int) *dnsmessage.Resource {
	if len(clientIP) == 0 && padding == 0 {
		return nil
	}

	const EDNS0SUBNET = 0x8
	const EDNS0PADDING = 0xc

	opt := new(dnsmessage.Resource)
	common.Must(opt.Header.SetEDNS0(1350, 0xfe00, true))
	body := dnsmessage.OPTResource{}
	opt.Body = &body

	if len(clientIP) != 0 {
		var netmask int
		var family uint16

		if len(clientIP) == 4 {
			family = 1
			netmask = 24 // 24 for IPV4, 96 for IPv6
		} else {
			family = 2
			netmask = 96
		}

		b := make([]byte, 4)
		binary.BigEndian.PutUint16(b[0:], family)
		b[2] = byte(netmask)
		b[3] = 0
		switch family {
		case 1:
			ip := clientIP.To4().Mask(net.CIDRMask(netmask, net.IPv4len*8))
			needLength := (netmask + 8 - 1) / 8 // division rounding up
			b = append(b, ip[:needLength]...)
		case 2:
			ip := clientIP.Mask(net.CIDRMask(netmask, net.IPv6len*8))
			needLength := (netmask + 8 - 1) / 8 // division rounding up
			b = append(b, ip[:needLength]...)
		}

		body.Options = append(body.Options,
			dnsmessage.Option{
				Code: EDNS0SUBNET,
				Data: b,
			})
	}

	if padding != 0 {
		body.Options = append(body.Options,
			dnsmessage.Option{
				Code: EDNS0PADDING,
				Data: make([]byte, padding),
			})
	}

	return opt
}

func buildReqMsgs(domain string, option dns_feature.IPOption, reqIDGen func() uint16, reqOpts *dnsmessage.Resource) []*dnsRequest {
	qA := dnsmessage.Question{
		Name:  dnsmessage.MustNewName(domain),
		Type:  dnsmessage.TypeA,
		Class: dnsmessage.ClassINET,
	}

	qAAAA := dnsmessage.Question{
		Name:  dnsmessage.MustNewName(domain),
		Type:  dnsmessage.TypeAAAA,
		Class: dnsmessage.ClassINET,
	}

	var reqs []*dnsRequest
	now := time.Now()

	if option.IPv4Enable {
		msg := new(dnsmessage.Message)
		msg.Header.ID = reqIDGen()
		msg.Header.RecursionDesired = true
		msg.Questions = []dnsmessage.Question{qA}
		if reqOpts != nil {
			msg.Additionals = append(msg.Additionals, *reqOpts)
		}
		reqs = append(reqs, &dnsRequest{
			reqType: dnsmessage.TypeA,
			domain:  domain,
			start:   now,
			msg:     msg,
		})
	}

	if option.IPv6Enable {
		msg := new(dnsmessage.Message)
		msg.Header.ID = reqIDGen()
		msg.Header.RecursionDesired = true
		msg.Questions = []dnsmessage.Question{qAAAA}
		if reqOpts != nil {
			msg.Additionals = append(msg.Additionals, *reqOpts)
		}
		reqs = append(reqs, &dnsRequest{
			reqType: dnsmessage.TypeAAAA,
			domain:  domain,
			start:   now,
			msg:     msg,
		})
	}

	return reqs
}

// parseResponse parses DNS answers from the returned payload
func parseResponse(payload []byte) (*IPRecord, error) {
	var parser dnsmessage.Parser
	h, err := parser.Start(payload)
	if err != nil {
		return nil, errors.New("failed to parse DNS response").Base(err).AtWarning()
	}
	if err := parser.SkipAllQuestions(); err != nil {
		return nil, errors.New("failed to skip questions in DNS response").Base(err).AtWarning()
	}

	now := time.Now()
	ipRecord := &IPRecord{
		ReqID:     h.ID,
		RCode:     h.RCode,
		Expire:    now.Add(time.Second * dns_feature.DefaultTTL),
		RawHeader: &h,
	}

L:
	for {
		ah, err := parser.AnswerHeader()
		if err != nil {
			if err != dnsmessage.ErrSectionDone {
				errors.LogInfoInner(context.Background(), err, "failed to parse answer section for domain: ", ah.Name.String())
			}
			break
		}

		ttl := ah.TTL
		if ttl == 0 {
			ttl = 1
		}
		expire := now.Add(time.Duration(ttl) * time.Second)
		if ipRecord.Expire.After(expire) {
			ipRecord.Expire = expire
		}

		switch ah.Type {
		case dnsmessage.TypeA:
			ans, err := parser.AResource()
			if err != nil {
				errors.LogInfoInner(context.Background(), err, "failed to parse A record for domain: ", ah.Name)
				break L
			}
			ipRecord.IP = append(ipRecord.IP, net.IPAddress(ans.A[:]).IP())
		case dnsmessage.TypeAAAA:
			ans, err := parser.AAAAResource()
			if err != nil {
				errors.LogInfoInner(context.Background(), err, "failed to parse AAAA record for domain: ", ah.Name)
				break L
			}
			newIP := net.IPAddress(ans.AAAA[:]).IP()
			if len(newIP) == net.IPv6len {
				ipRecord.IP = append(ipRecord.IP, newIP)
			}
		default:
			if err := parser.SkipAnswer(); err != nil {
				errors.LogInfoInner(context.Background(), err, "failed to skip answer")
				break L
			}
			continue
		}
	}

	return ipRecord, nil
}

// toDnsContext create a new background context with parent inbound, session and dns log
func toDnsContext(ctx context.Context, addr string) context.Context {
	dnsCtx := core.ToBackgroundDetachedContext(ctx)
	if inbound := session.InboundFromContext(ctx); inbound != nil {
		dnsCtx = session.ContextWithInbound(dnsCtx, inbound)
	}
	dnsCtx = session.ContextWithContent(dnsCtx, session.ContentFromContext(ctx))
	dnsCtx = log.ContextWithAccessMessage(dnsCtx, &log.AccessMessage{
		From:   "DNS",
		To:     addr,
		Status: log.AccessAccepted,
		Reason: "",
	})
	return dnsCtx
}
