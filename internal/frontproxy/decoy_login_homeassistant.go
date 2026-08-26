package frontproxy

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// haLoginFlowPath is Home Assistant's real login-flow endpoint family:
// POST /auth/login_flow starts a flow, POST /auth/login_flow/{flow_id}
// submits a step. Confirmed from home-assistant/core's
// homeassistant/components/auth/login_flow.py module docstring.
const haLoginFlowPath = "/auth/login_flow"

// haFlowForm is the real shape of a login-flow step from that same
// docstring: a fresh flow has Errors nil, a rejected step has
// {"base":"invalid_auth"} -- HA's own data_entry_flow convention for a
// failed step, not a distinct error response type.
type haFlowForm struct {
	Type       string            `json:"type"`
	FlowID     string            `json:"flow_id"`
	Handler    [2]any            `json:"handler"`
	StepID     string            `json:"step_id"`
	DataSchema []haSchemaField   `json:"data_schema"`
	Errors     map[string]string `json:"errors"`
}

type haSchemaField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func haNewFlowID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func haDataSchema() []haSchemaField {
	return []haSchemaField{{Name: "username", Type: "string"}, {Name: "password", Type: "string"}}
}

func writeHAFlow(w http.ResponseWriter, status int, f haFlowForm) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(f)
}

// homeAssistantHandler mimics /auth/login_flow's two real steps and the real
// IP-ban middleware in front of them (homeassistant/components/http/ban.go):
// once banned, every request -- not just the login endpoint -- gets 403,
// checked before anything else. Real HA bans persist until an admin clears
// them (written to ip_bans.yaml, no auto-expiry); this mock approximates
// that with an effectively-unbounded ban duration (see the registration in
// decoy_login.go) rather than real disk persistence, since it only needs to
// survive one running process, not a restart.
//
// Two details are informed inference, not a re-read of the exact handler:
// the 400 status on a rejected step follows from log_invalid_auth's own
// "status >= 400" check in ban.py; "homeassistant" as the handler's provider
// id is this fork's own local auth provider's real internal type string,
// not the "insecure_example" demo value the docstring itself shows.
func homeAssistantHandler(page http.Handler, tracker *loginAttempts, threshold int, ban time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientKey(r)
		if locked, _ := tracker.check(key); locked {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == haLoginFlowPath:
			writeHAFlow(w, http.StatusOK, haFlowForm{
				Type:       "form",
				FlowID:     haNewFlowID(),
				Handler:    [2]any{"homeassistant", nil},
				StepID:     "init",
				DataSchema: haDataSchema(),
			})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, haLoginFlowPath+"/"):
			flowID := strings.TrimPrefix(r.URL.Path, haLoginFlowPath+"/")
			tracker.fail(key, threshold, ban)
			writeHAFlow(w, http.StatusBadRequest, haFlowForm{
				Type:       "form",
				FlowID:     flowID,
				Handler:    [2]any{"homeassistant", nil},
				StepID:     "init",
				DataSchema: haDataSchema(),
				Errors:     map[string]string{"base": "invalid_auth"},
			})
		default:
			page.ServeHTTP(w, r)
		}
	})
}
