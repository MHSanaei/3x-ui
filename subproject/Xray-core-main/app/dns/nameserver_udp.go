package dns

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol/dns"
	udp_proto "github.com/xtls/xray-core/common/protocol/udp"
	"github.com/xtls/xray-core/common/task"
	dns_feature "github.com/xtls/xray-core/features/dns"
	"github.com/xtls/xray-core/features/routing"
	"github.com/xtls/xray-core/transport/internet/udp"
	"golang.org/x/net/dns/dnsmessage"
)

// ClassicNameServer implemented traditional UDP DNS.
type ClassicNameServer struct {
	sync.RWMutex
	cacheController *CacheController
	address         *net.Destination
	requests        map[uint16]*udpDnsRequest
	udpServer       *udp.Dispatcher
	requestsCleanup *task.Periodic
	reqID           uint32
	clientIP        net.IP
}

type udpDnsRequest struct {
	dnsRequest
	ctx context.Context
}

// NewClassicNameServer creates udp server object for remote resolving.
func NewClassicNameServer(address net.Destination, dispatcher routing.Dispatcher, disableCache bool, serveStale bool, serveExpiredTTL uint32, clientIP net.IP) *ClassicNameServer {
	// default to 53 if unspecific
	if address.Port == 0 {
		address.Port = net.Port(53)
	}

	s := &ClassicNameServer{
		cacheController: NewCacheController(strings.ToUpper(address.String()), disableCache, serveStale, serveExpiredTTL),
		address:         &address,
		requests:        make(map[uint16]*udpDnsRequest),
		clientIP:        clientIP,
	}
	s.requestsCleanup = &task.Periodic{
		Interval: time.Minute,
		Execute:  s.RequestsCleanup,
	}
	s.udpServer = udp.NewDispatcher(dispatcher, s.HandleResponse)

	errors.LogInfo(context.Background(), "DNS: created UDP client initialized for ", address.NetAddr())
	return s
}

// Name implements Server.
func (s *ClassicNameServer) Name() string {
	return s.cacheController.name
}

// IsDisableCache implements Server.
func (s *ClassicNameServer) IsDisableCache() bool {
	return s.cacheController.disableCache
}

// RequestsCleanup clears expired items from cache
func (s *ClassicNameServer) RequestsCleanup() error {
	now := time.Now()
	s.Lock()
	defer s.Unlock()

	if len(s.requests) == 0 {
		return errors.New(s.Name(), " nothing to do. stopping...")
	}

	for id, req := range s.requests {
		if req.expire.Before(now) {
			delete(s.requests, id)
		}
	}

	if len(s.requests) == 0 {
		s.requests = make(map[uint16]*udpDnsRequest)
	}

	return nil
}

// HandleResponse handles udp response packet from remote DNS server.
func (s *ClassicNameServer) HandleResponse(ctx context.Context, packet *udp_proto.Packet) {
	payload := packet.Payload
	ipRec, err := parseResponse(payload.Bytes())
	payload.Release()
	if err != nil {
		errors.LogErrorInner(ctx, err, s.Name(), " fail to parse responded DNS udp")
		return
	}

	s.Lock()
	id := ipRec.ReqID
	req, ok := s.requests[id]
	if ok {
		// remove the pending request
		delete(s.requests, id)
	}
	s.Unlock()
	if !ok {
		errors.LogErrorInner(ctx, err, s.Name(), " cannot find the pending request")
		return
	}

	// if truncated, retry with EDNS0 option(udp payload size: 1350)
	if ipRec.RawHeader.Truncated {
		// if already has EDNS0 option, no need to retry
		if len(req.msg.Additionals) == 0 {
			// copy necessary meta data from original request
			// and add EDNS0 option
			opt := new(dnsmessage.Resource)
			common.Must(opt.Header.SetEDNS0(1350, 0xfe00, true))
			opt.Body = &dnsmessage.OPTResource{}
			newMsg := *req.msg
			newReq := *req
			newMsg.Additionals = append(newMsg.Additionals, *opt)
			newMsg.ID = s.newReqID()
			newReq.msg = &newMsg
			s.addPendingRequest(&newReq)
			b, _ := dns.PackMessage(newReq.msg)
			copyDest := net.UDPDestination(s.address.Address, s.address.Port)
			b.UDP = &copyDest
			s.udpServer.Dispatch(toDnsContext(newReq.ctx, s.address.String()), *s.address, b)
			return
		}
	}

	s.cacheController.updateRecord(&req.dnsRequest, ipRec)
}

func (s *ClassicNameServer) newReqID() uint16 {
	return uint16(atomic.AddUint32(&s.reqID, 1))
}

func (s *ClassicNameServer) addPendingRequest(req *udpDnsRequest) {
	s.Lock()
	id := req.msg.ID
	req.expire = time.Now().Add(time.Second * 8)
	s.requests[id] = req
	s.Unlock()
	common.Must(s.requestsCleanup.Start())
}

// getCacheController implements CachedNameserver.
func (s *ClassicNameServer) getCacheController() *CacheController {
	return s.cacheController
}

// sendQuery implements CachedNameserver.
func (s *ClassicNameServer) sendQuery(ctx context.Context, noResponseErrCh chan<- error, fqdn string, option dns_feature.IPOption) {
	errors.LogInfo(ctx, s.Name(), " querying DNS for: ", fqdn)

	reqs := buildReqMsgs(fqdn, option, s.newReqID, genEDNS0Options(s.clientIP, 0))

	for _, req := range reqs {
		udpReq := &udpDnsRequest{
			dnsRequest: *req,
			ctx:        ctx,
		}
		s.addPendingRequest(udpReq)
		b, err := dns.PackMessage(req.msg)
		if err != nil {
			errors.LogErrorInner(ctx, err, "failed to pack dns query")
			if noResponseErrCh != nil {
				noResponseErrCh <- err
			}
			return
		}
		copyDest := net.UDPDestination(s.address.Address, s.address.Port)
		b.UDP = &copyDest
		s.udpServer.Dispatch(toDnsContext(ctx, s.address.String()), *s.address, b)
	}
}

// QueryIP implements Server.
func (s *ClassicNameServer) QueryIP(ctx context.Context, domain string, option dns_feature.IPOption) ([]net.IP, uint32, error) {
	return queryIP(ctx, s, domain, option)
}
