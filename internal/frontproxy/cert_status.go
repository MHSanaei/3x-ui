package frontproxy

import (
	"context"
	"sync"
	"time"

	"github.com/caddyserver/certmagic"
)

// CertState is where the reverse proxy's TLS certificate currently stands,
// surfaced to the settings UI so a slow or failing ACME round is visible
// instead of a silent gap between clicking Start and the door actually
// working.
type CertState string

const (
	// CertStateIdle means no certificate activity has been observed this
	// process's lifetime (manual mode, or auto mode not yet started).
	CertStateIdle CertState = ""
	// CertStateObtaining means CertMagic is actively trying to get or renew
	// a certificate right now.
	CertStateObtaining CertState = "obtaining"
	// CertStateObtained means a certificate is on hand and servable, whether
	// it was just issued, just renewed, already existed from a previous run,
	// or was loaded from admin-provided files in manual mode.
	CertStateObtained CertState = "obtained"
	// CertStateFailed means the most recent attempt did not produce a usable
	// certificate.
	CertStateFailed CertState = "failed"
)

// CertStatus is a point-in-time snapshot of CertState plus whatever detail
// goes with it -- an expiry once obtained, or the failure reason. NotAfter is
// a pointer so the zero (unknown) case is actually omitted from JSON:
// encoding/json's omitempty does not detect a zero-value time.Time on a
// plain (non-pointer) struct field, so this would otherwise serialize as the
// confusing literal "0001-01-01T00:00:00Z" instead of being left out.
type CertStatus struct {
	State    CertState  `json:"state"`
	Domain   string     `json:"domain,omitempty"`
	Error    string     `json:"error,omitempty"`
	NotAfter *time.Time `json:"notAfter,omitempty"`
}

var (
	certStatusMu sync.RWMutex
	certStatus   CertStatus
)

// CurrentCertStatus returns the last known certificate status. Safe for
// concurrent use; called from the settings UI's status poll.
func CurrentCertStatus() CertStatus {
	certStatusMu.RLock()
	defer certStatusMu.RUnlock()
	return certStatus
}

func setCertStatus(s CertStatus) {
	certStatusMu.Lock()
	certStatus = s
	certStatusMu.Unlock()
}

// resetCertStatus clears any observed state, used when the reverse proxy
// stops -- a stale "obtaining" spinner with nothing left to finish it would
// otherwise spin forever in the UI.
func resetCertStatus() { setCertStatus(CertStatus{}) }

// certOnEvent wires CertMagic's OnEvent hook to certStatus. cfg is the same
// *certmagic.Config the caller is about to call ManageAsync on -- captured by
// closure rather than read back out of the event data, which carries only
// paths/identifiers, never the config itself. CertMagic emits the same
// events for a renewal as for the first issuance; this fork treats both the
// same way, since "is the certificate currently valid and until when"
// matters the same regardless of which one is happening.
func certOnEvent(cfg *certmagic.Config, domain string) func(ctx context.Context, event string, data map[string]any) error {
	return func(_ context.Context, event string, data map[string]any) error {
		switch event {
		case "cert_obtaining":
			setCertStatus(CertStatus{State: CertStateObtaining, Domain: domain})
		case "cert_obtained":
			status := CertStatus{State: CertStateObtained, Domain: domain}
			if notAfter, err := currentCertNotAfter(cfg, domain); err == nil {
				status.NotAfter = &notAfter
			}
			setCertStatus(status)
		case "cert_failed":
			msg := ""
			if e, ok := data["error"].(error); ok {
				msg = e.Error()
			}
			setCertStatus(CertStatus{State: CertStateFailed, Domain: domain, Error: msg})
		}
		return nil
	}
}

// currentCertNotAfter loads the domain's certificate straight from CertMagic's
// storage (not a synthetic TLS handshake -- constructing a fake
// tls.ClientHelloInfo to call GetCertificate risks a nil-pointer panic deep
// inside CertMagic's handshake logging, which dereferences ClientHelloInfo.Conn
// unconditionally on some paths). CacheManagedCertificate is the documented,
// concurrency-safe way to do this and guarantees the returned Certificate's
// Leaf is parsed.
func currentCertNotAfter(cfg *certmagic.Config, domain string) (time.Time, error) {
	cert, err := cfg.CacheManagedCertificate(context.Background(), domain)
	if err != nil {
		return time.Time{}, err
	}
	if cert.Leaf == nil {
		return time.Time{}, errNoParsedLeaf
	}
	return cert.Leaf.NotAfter, nil
}

var errNoParsedLeaf = certLeafError("certificate has no parsed leaf")

type certLeafError string

func (e certLeafError) Error() string { return string(e) }
