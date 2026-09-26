package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// Sponsor is one paid placement published in the repo's sponsors.json.
type Sponsor struct {
	ID     string            `json:"id" example:"acme-2026-10"`
	Name   string            `json:"name" example:"Acme VPS"`
	Enable *bool             `json:"enable,omitempty" example:"true"`
	Slots  []string          `json:"slots"`
	Until  time.Time         `json:"until" example:"2026-11-01T00:00:00Z"`
	Logo   string            `json:"logo,omitempty" example:"/sponsors/logo/acme.png"`
	Title  map[string]string `json:"title"`
	Text   map[string]string `json:"text"`
	Link   string            `json:"link" example:"https://acme.example/?utm_source=3x-ui"`
}

// SponsorList is the active sponsor set plus the contact link for new sponsors.
type SponsorList struct {
	Contact  string    `json:"contact,omitempty" example:"https://t.me/example"`
	Sponsors []Sponsor `json:"sponsors"`
}

const (
	sponsorsTTL      = time.Hour
	sponsorsErrTTL   = 10 * time.Minute
	maxSponsorsBytes = 256 << 10
	maxLogoBytes     = 256 << 10
	maxSidebarSlots  = 3
	localSponsorsDir = "../sponsors/3X"
	sponsorLogoPath  = "/sponsors/logo/"
)

// ErrSponsorLogoUnknown rejects logo names not used by an active sponsor.
var ErrSponsorLogoUnknown = errors.New("unknown sponsor logo")

type sponsorLogo struct {
	data        []byte
	contentType string
	err         error
	retryAt     time.Time
}

var (
	sponsorsURL     = "https://sponsors.sanaei.dev/3X/sponsors.json"
	sponsorLogoBase = "https://sponsors.sanaei.dev/3X/logos/"
	sponsorNow      = time.Now

	// The panel proxies logos from sponsorLogoBase: CSP stays 'self' and no third party sees admin IPs.
	sponsorLogoRe = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]*\.(png|webp|jpg)$`)
	sponsorSlots  = map[string]bool{"dashboard": true, "sidebar": true, "page": true, "login": true}

	sponsorsMu      sync.Mutex
	sponsorsRaw     *SponsorList
	sponsorsErr     error
	sponsorsRetryAt time.Time

	logosMu sync.Mutex
	logos   = map[string]sponsorLogo{}
)

// GetSponsors returns the currently active sponsors. The remote file is cached,
// but expiry is re-checked on every call so a slot ends exactly at Until.
func (s *PanelService) GetSponsors() (*SponsorList, error) {
	raw, err := cachedSponsors()
	if err != nil {
		return nil, err
	}
	return activeSponsors(raw, sponsorNow()), nil
}

func cachedSponsors() (*SponsorList, error) {
	sponsorsMu.Lock()
	defer sponsorsMu.Unlock()
	now := sponsorNow()
	if !config.IsDebug() && now.Before(sponsorsRetryAt) {
		return sponsorsRaw, sponsorsErr
	}
	list, err := fetchSponsors()
	switch {
	case err == nil:
		sponsorsRaw, sponsorsErr, sponsorsRetryAt = list, nil, now.Add(sponsorsTTL)
	case sponsorsRaw != nil:
		// An upstream blip keeps the last good list, so paid slots do not blink out.
		logger.Debug("sponsors refresh failed, keeping last list:", err)
		sponsorsRetryAt = now.Add(sponsorsErrTTL)
	default:
		sponsorsErr, sponsorsRetryAt = err, now.Add(sponsorsErrTTL)
	}
	return sponsorsRaw, sponsorsErr
}

// GetSponsorLogo returns the image bytes for a logo of a currently active sponsor.
func (s *PanelService) GetSponsorLogo(name string) ([]byte, string, error) {
	// Validated here, not only via list membership, so name can never carry a path or URL.
	if !sponsorLogoRe.MatchString(name) {
		return nil, "", ErrSponsorLogoUnknown
	}
	sponsors, err := s.GetSponsors()
	if err != nil {
		return nil, "", err
	}
	if !slices.ContainsFunc(sponsors.Sponsors, func(sp Sponsor) bool { return sp.Logo == sponsorLogoPath+name }) {
		return nil, "", ErrSponsorLogoUnknown
	}
	logosMu.Lock()
	defer logosMu.Unlock()
	now := sponsorNow()
	l, ok := logos[name]
	if ok && !config.IsDebug() && now.Before(l.retryAt) {
		return l.data, l.contentType, l.err
	}
	// Failures are cached too: this route is public and each miss is an outbound fetch.
	data, contentType, err := fetchSponsorLogo(name)
	switch {
	case err == nil:
		l = sponsorLogo{data: data, contentType: contentType, retryAt: now.Add(sponsorsTTL)}
	case l.data != nil:
		l.retryAt = now.Add(sponsorsErrTTL)
	default:
		l = sponsorLogo{err: err, retryAt: now.Add(sponsorsErrTTL)}
	}
	logos[name] = l
	return l.data, l.contentType, l.err
}

func fetchSponsorLogo(name string) ([]byte, string, error) {
	data, err := readSponsorSource(sponsorLogoBase+name, filepath.Join(localSponsorsDir, "logos", name), maxLogoBytes)
	if err != nil {
		return nil, "", err
	}
	contentType := http.DetectContentType(data)
	switch contentType {
	case "image/png", "image/webp", "image/jpeg":
		return data, contentType, nil
	default:
		return nil, "", fmt.Errorf("sponsor logo %s has content type %s", name, contentType)
	}
}

// readSponsorSource reads a sibling checkout of MHSanaei/sponsors under XUI_DEBUG so
// sponsor edits can be previewed locally before they are pushed.
func readSponsorSource(url, localPath string, limit int) ([]byte, error) {
	if !config.IsDebug() {
		return fetchLimited(url, limit)
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", localPath, limit)
	}
	return data, nil
}

func fetchLimited(url string, limit int) ([]byte, error) {
	client := (&service.SettingService{}).NewProxiedHTTPClient(10 * time.Second)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s returned status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(body) > limit {
		return nil, fmt.Errorf("%s exceeds %d bytes", url, limit)
	}
	return body, nil
}

func fetchSponsors() (*SponsorList, error) {
	body, err := readSponsorSource(sponsorsURL, filepath.Join(localSponsorsDir, "sponsors.json"), maxSponsorsBytes)
	if err != nil {
		return nil, err
	}
	var list SponsorList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

func activeSponsors(raw *SponsorList, now time.Time) *SponsorList {
	out := &SponsorList{Sponsors: []Sponsor{}}
	if raw == nil {
		return out
	}
	if strings.HasPrefix(raw.Contact, "https://") {
		out.Contact = raw.Contact
	}
	sidebarTaken := 0
	for _, sp := range raw.Sponsors {
		// A missing enable counts as on, so a forgotten field never hides a paid slot.
		disabled := sp.Enable != nil && !*sp.Enable
		if disabled || sp.ID == "" || !now.Before(sp.Until) || !strings.HasPrefix(sp.Link, "https://") {
			continue
		}
		// A bad logo name drops only the logo; the paid slot still renders with its initial.
		if sponsorLogoRe.MatchString(sp.Logo) {
			sp.Logo = sponsorLogoPath + sp.Logo
		} else {
			sp.Logo = ""
		}
		slots := make([]string, 0, len(sp.Slots))
		for _, slot := range sp.Slots {
			// The sidebar rotates, so capping it keeps each paid card on screen long enough.
			if !sponsorSlots[slot] || (slot == "sidebar" && sidebarTaken >= maxSidebarSlots) {
				continue
			}
			slots = append(slots, slot)
		}
		if len(slots) == 0 {
			continue
		}
		if slices.Contains(slots, "sidebar") {
			sidebarTaken++
		}
		sp.Slots = slots
		out.Sponsors = append(out.Sponsors, sp)
	}
	return out
}
