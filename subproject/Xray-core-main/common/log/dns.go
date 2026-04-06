package log

import (
	"net"
	"strings"
	"time"
)

type DNSLog struct {
	Server  string
	Domain  string
	Result  []net.IP
	Status  dnsStatus
	Elapsed time.Duration
	Error   error
}

func (l *DNSLog) String() string {
	builder := &strings.Builder{}

	// Server got answer: domain -> [ip1, ip2] 23ms
	builder.WriteString(l.Server)
	builder.WriteString(" ")
	builder.WriteString(string(l.Status))
	builder.WriteString(" ")
	builder.WriteString(l.Domain)
	builder.WriteString(" -> [")
	builder.WriteString(joinNetIP(l.Result))
	builder.WriteString("]")

	if l.Elapsed > 0 {
		builder.WriteString(" ")
		builder.WriteString(l.Elapsed.String())
	}
	if l.Error != nil {
		builder.WriteString(" <")
		builder.WriteString(l.Error.Error())
		builder.WriteString(">")
	}
	return builder.String()
}

type dnsStatus string

var (
	DNSQueried        = dnsStatus("got answer:")
	DNSCacheHit       = dnsStatus("cache HIT:")
	DNSCacheOptimiste = dnsStatus("cache OPTIMISTE:")
)

func joinNetIP(ips []net.IP) string {
	if len(ips) == 0 {
		return ""
	}
	sips := make([]string, 0, len(ips))
	for _, ip := range ips {
		sips = append(sips, ip.String())
	}
	return strings.Join(sips, ", ")
}
