package integration

import (
	"context"
	"encoding/json"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/crypto/nodetoken"
	piaprotocol "github.com/mhsanaei/3x-ui/v3/internal/pia"
	"github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

var piaTokenAAD = []byte("settings/pia_token")

type PiaService struct {
	service.SettingService
	Auth      piaprotocol.Authenticator
	Catalog   *piaprotocol.Catalog
	Registrar piaprotocol.Registrar
}

type piaStored struct {
	Username       string `json:"username"`
	Token          string `json:"token"`
	TokenExpiresAt int64  `json:"tokenExpiresAt"`
}

type PiaAccountView struct {
	Username    string `json:"username"`
	AccountHint string `json:"accountHint"`
}

type PiaCountryView struct {
	Code string `json:"code"`
}

type PiaRegionView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PiaServerView struct {
	Hostname   string `json:"hostname"`
	IP         string `json:"ip"`
	RegionID   string `json:"regionId"`
	RegionName string `json:"regionName"`
}

type PiaServersView struct {
	Regions []PiaRegionView `json:"regions"`
	Servers []PiaServerView `json:"servers"`
}

type PiaKeyView struct {
	Tag       string `json:"tag"`
	Hostname  string `json:"hostname"`
	SecretKey string `json:"secretKey"`
	Address   string `json:"address"`
	PublicKey string `json:"publicKey"`
	Endpoint  string `json:"endpoint"`
}

func NewPiaService() *PiaService {
	return &PiaService{
		Auth:      piaprotocol.NewAuthClient(piaprotocol.DefaultTokenEndpoint),
		Catalog:   piaprotocol.NewCatalog(piaprotocol.NewCatalogClient(piaprotocol.DefaultServerListEndpoint, piaprotocol.EmbeddedServerListPublicKey)),
		Registrar: piaprotocol.NewRegistrationClient(piaprotocol.EmbeddedPIACA),
	}
}

func (s *PiaService) Login(username, password string) (*PiaAccountView, error) {
	tok, err := s.Auth.Authenticate(context.Background(), username, []byte(password))
	if err != nil {
		return nil, err
	}
	stored := piaStored{
		Username:       strings.TrimSpace(username),
		Token:          string(tok.Value),
		TokenExpiresAt: tok.ExpiresAt.Unix(),
	}
	if err := s.saveStored(stored); err != nil {
		return nil, err
	}
	return accountView(stored.Username), nil
}

func (s *PiaService) GetPiaData() (*PiaAccountView, error) {
	stored, err := s.loadStored()
	if err != nil {
		return nil, err
	}
	if stored == nil || stored.Token == "" {
		return nil, nil
	}
	return accountView(stored.Username), nil
}

func (s *PiaService) DelPiaData() error {
	return s.SetPia("")
}

func (s *PiaService) GetCountries() ([]PiaCountryView, error) {
	regions, err := s.regions()
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	out := make([]PiaCountryView, 0)
	for _, region := range regions {
		code := strings.ToUpper(strings.TrimSpace(region.CountryCode))
		if !validCountryCode(code) {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, PiaCountryView{Code: code})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func (s *PiaService) GetServers(countryCode string) (*PiaServersView, error) {
	code := strings.ToUpper(strings.TrimSpace(countryCode))
	if !validCountryCode(code) {
		return nil, piaprotocol.NewError(piaprotocol.CodeInvalidInput, "Select a country.")
	}
	regions, err := s.regions()
	if err != nil {
		return nil, err
	}
	view := &PiaServersView{Regions: []PiaRegionView{}, Servers: []PiaServerView{}}
	for _, region := range regions {
		if strings.ToUpper(strings.TrimSpace(region.CountryCode)) != code {
			continue
		}
		view.Regions = append(view.Regions, PiaRegionView{ID: region.ID, Name: region.Name})
		for _, server := range region.WireGuard {
			view.Servers = append(view.Servers, PiaServerView{
				Hostname:   server.Hostname,
				IP:         server.IP.String(),
				RegionID:   region.ID,
				RegionName: region.Name,
			})
		}
	}
	sort.Slice(view.Regions, func(i, j int) bool { return view.Regions[i].Name < view.Regions[j].Name })
	return view, nil
}

func (s *PiaService) AddKey(hostname string) (*PiaKeyView, error) {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return nil, piaprotocol.NewError(piaprotocol.CodeInvalidInput, "Select a PIA server.")
	}
	stored, err := s.loadStored()
	if err != nil {
		return nil, err
	}
	if stored == nil || stored.Token == "" {
		return nil, piaprotocol.NewError(piaprotocol.CodeTokenRejected, "Sign in with a PIA account first.")
	}
	if stored.TokenExpiresAt > 0 && time.Now().Unix() >= stored.TokenExpiresAt {
		return nil, piaprotocol.NewError(piaprotocol.CodeTokenRejected, "The PIA token has expired. Sign in again.")
	}
	region, server, err := s.findServer(hostname)
	if err != nil {
		return nil, err
	}
	priv, pub, err := wireguard.GenerateWireguardKeypair()
	if err != nil {
		return nil, err
	}
	reg, err := s.Registrar.RegisterKey(context.Background(), server, stored.Token, pub)
	if err != nil {
		return nil, err
	}
	return &PiaKeyView{
		Tag:       piaOutboundTag(region.ID, server.Hostname),
		Hostname:  server.Hostname,
		SecretKey: priv,
		Address:   reg.PeerIP.String(),
		PublicKey: reg.ServerKey,
		Endpoint:  net.JoinHostPort(reg.ServerIP.String(), strconv.Itoa(int(reg.ServerPort))),
	}, nil
}

func (s *PiaService) regions() ([]piaprotocol.Region, error) {
	if s.Catalog == nil {
		return nil, piaprotocol.NewError(piaprotocol.CodeCatalogUnavailable, "The PIA server list is not available.")
	}
	regions, _, err := s.Catalog.ListRegions(context.Background())
	return regions, err
}

func (s *PiaService) findServer(hostname string) (piaprotocol.Region, piaprotocol.WireGuardServer, error) {
	regions, err := s.regions()
	if err != nil {
		return piaprotocol.Region{}, piaprotocol.WireGuardServer{}, err
	}
	for _, region := range regions {
		for _, server := range region.WireGuard {
			if server.Hostname == hostname {
				return region, server, nil
			}
		}
	}
	return piaprotocol.Region{}, piaprotocol.WireGuardServer{}, piaprotocol.NewError(piaprotocol.CodeServerNotFound, "The selected PIA server was not found.")
}

func piaOutboundTag(regionID, hostname string) string {
	region := piaTagPart(regionID, false)
	server := piaTagPart(hostname, true)
	if region == "" {
		return "pia-" + server
	}
	return "pia-" + region + "-" + server
}

func piaTagPart(s string, stripDomain bool) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if stripDomain {
		if i := strings.IndexByte(s, '.'); i > 0 {
			s = s[:i]
		}
	}
	return strings.ReplaceAll(s, "_", "-")
}

func (s *PiaService) saveStored(stored piaStored) error {
	enc, err := nodetoken.EncryptBound(piaTokenAAD, stored.Token)
	if err != nil {
		return err
	}
	stored.Token = enc
	raw, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	return s.SetPia(string(raw))
}

func (s *PiaService) loadStored() (*piaStored, error) {
	raw, err := s.GetPia()
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil, err
	}
	var stored piaStored
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, err
	}
	atRest := stored.Token
	if atRest == "" {
		return &stored, nil
	}
	if nodetoken.IsEncrypted(atRest) && !nodetoken.Enabled() {
		return nil, piaprotocol.NewError(piaprotocol.CodeTokenRejected, "The PIA token is encrypted but NODE_TOKEN_ENCRYPTION is off. Sign in again.")
	}
	plain, err := nodetoken.DecryptBound(piaTokenAAD, atRest)
	if err != nil {
		return nil, err
	}
	stored.Token = plain
	if nodetoken.Enabled() && !nodetoken.IsEncrypted(atRest) {
		if err := s.saveStored(stored); err != nil {
			return nil, err
		}
	}
	return &stored, nil
}

func accountView(username string) *PiaAccountView {
	return &PiaAccountView{Username: username, AccountHint: piaAccountHint(username)}
}

func piaAccountHint(username string) string {
	u := strings.TrimSpace(username)
	if len(u) <= 4 {
		return strings.Repeat("*", len(u))
	}
	return u[:2] + strings.Repeat("*", len(u)-4) + u[len(u)-2:]
}

func validCountryCode(code string) bool {
	if len(code) != 2 {
		return false
	}
	return code[0] >= 'A' && code[0] <= 'Z' && code[1] >= 'A' && code[1] <= 'Z'
}
