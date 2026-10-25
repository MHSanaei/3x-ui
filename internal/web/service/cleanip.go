package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CleanIpNetwork is the network (ISP) a panel visitor is on. The clean IP
// scanner runs in that visitor's browser, so its result belongs to this ISP.
type CleanIpNetwork struct {
	Ip      string `json:"ip" example:"5.160.10.20"`
	Asn     int    `json:"asn" example:"197207"`
	Org     string `json:"org" example:"Mobile Communication Company of Iran PLC"`
	Country string `json:"country" example:"IR"`
	Isp     string `json:"isp" example:"MCI"`
}

// cleanIpIspNames are the short ISP names Iranian panels already use in config
// names (the same table as Hiddify's), keyed by AS number.
var cleanIpIspNames = map[int]string{
	58224: "MKH", 197207: "MCI", 12880: "ITC", 49666: "ITC", 44244: "MTN", 57218: "RTL",
	16322: "PRS", 56402: "HWB", 41689: "AST", 43754: "AST", 31549: "SHT", 205647: "SHT",
	50810: "MBT", 39308: "ASK", 205207: "RSP", 42337: "RSP", 25184: "AFR", 394510: "ZTL",
	206065: "ZTL", 49100: "PSM", 44208: "PSG",
}

// https://www.cloudflare.com/ips-v6/ (IPv4 in cleanIpCloudflareRanges)
var cleanIpCloudflareRangesV6 = []string{
	"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32", "2405:8100::/32",
	"2a06:98c0::/29", "2c0f:f248::/32",
}

var cleanIpCloudflareNets = func() []*net.IPNet {
	var nets []*net.IPNet
	for _, cidr := range append(append([]string{}, cleanIpCloudflareRanges...), cleanIpCloudflareRangesV6...) {
		if _, n, err := net.ParseCIDR(cidr); err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}()

// CleanIpVisitorIp is the address to look the ISP up for: a panel behind Cloudflare's
// proxy sees Cloudflare, so its CF-Connecting-IP counts, but only on Cloudflare's own hops.
func CleanIpVisitorIp(remote, cfConnectingIp string) string {
	client := net.ParseIP(strings.TrimSpace(cfConnectingIp))
	from := net.ParseIP(remote)
	if client == nil || from == nil {
		return remote
	}
	for _, n := range cleanIpCloudflareNets {
		if n.Contains(from) {
			return client.String()
		}
	}
	return remote
}

const cleanIpCacheTTL = time.Hour

type cleanIpCacheEntry struct {
	network CleanIpNetwork
	at      time.Time
}

var (
	cleanIpCacheMu sync.Mutex
	cleanIpCache   = map[string]cleanIpCacheEntry{}
)

type CleanIpService struct {
	settingService SettingService
}

// LookupNetwork finds the ISP of ip. A private or loopback address (the panel
// opened on its own machine) has no ISP and comes back with only Ip set.
func (s *CleanIpService) LookupNetwork(ctx context.Context, ip string) (*CleanIpNetwork, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, fmt.Errorf("not an IP address: %q", ip)
	}
	if parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsLinkLocalUnicast() {
		return &CleanIpNetwork{Ip: ip}, nil
	}
	cleanIpCacheMu.Lock()
	cached, ok := cleanIpCache[ip]
	cleanIpCacheMu.Unlock()
	if ok && time.Since(cached.at) < cleanIpCacheTTL {
		out := cached.network
		return &out, nil
	}

	client := s.settingService.NewProxiedHTTPClient(8 * time.Second)
	var errs []error
	for _, lookup := range []func(context.Context, *http.Client, string) (*CleanIpNetwork, error){
		lookupIpWhoIs, lookupIpApi,
	} {
		network, err := lookup(ctx, client, ip)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		network.Ip = ip
		network.Isp = cleanIpIspNames[network.Asn]
		cleanIpCacheMu.Lock()
		cleanIpCache[ip] = cleanIpCacheEntry{network: *network, at: time.Now()}
		cleanIpCacheMu.Unlock()
		return network, nil
	}
	return nil, errors.Join(errs...)
}

func getJSON(ctx context.Context, client *http.Client, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "3x-ui")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(out)
}

func lookupIpWhoIs(ctx context.Context, client *http.Client, ip string) (*CleanIpNetwork, error) {
	var body struct {
		Success     bool   `json:"success"`
		Message     string `json:"message"`
		CountryCode string `json:"country_code"`
		Connection  struct {
			Asn int    `json:"asn"`
			Org string `json:"org"`
			Isp string `json:"isp"`
		} `json:"connection"`
	}
	if err := getJSON(ctx, client, "https://ipwho.is/"+ip, &body); err != nil {
		return nil, err
	}
	if !body.Success || body.Connection.Asn == 0 {
		return nil, fmt.Errorf("ipwho.is: %s", body.Message)
	}
	org := body.Connection.Org
	if org == "" {
		org = body.Connection.Isp
	}
	return &CleanIpNetwork{Asn: body.Connection.Asn, Org: org, Country: body.CountryCode}, nil
}

func lookupIpApi(ctx context.Context, client *http.Client, ip string) (*CleanIpNetwork, error) {
	var body struct {
		Status      string `json:"status"`
		Message     string `json:"message"`
		CountryCode string `json:"countryCode"`
		As          string `json:"as"`
		Org         string `json:"org"`
		Isp         string `json:"isp"`
	}
	if err := getJSON(ctx, client, "http://ip-api.com/json/"+ip+"?fields=status,message,countryCode,as,org,isp", &body); err != nil {
		return nil, err
	}
	// "as" is "AS197207 Mobile Communication Company of Iran PLC"
	asField, _, _ := strings.Cut(body.As, " ")
	asn, err := strconv.Atoi(strings.TrimPrefix(asField, "AS"))
	if body.Status != "success" || err != nil || asn == 0 {
		return nil, fmt.Errorf("ip-api.com: %s", body.Message)
	}
	org := body.Isp
	if org == "" {
		org = body.Org
	}
	return &CleanIpNetwork{Asn: asn, Org: org, Country: body.CountryCode}, nil
}
