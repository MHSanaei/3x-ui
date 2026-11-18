package ha

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// MasterHeader carries the master's URL on every not-master answer.
const MasterHeader = "X-3xui-Master"

// PanelTarget picks the master's panel URL as the redirect target.
func PanelTarget(s Snapshot) string { return s.MasterPanelURL }

// SubTarget picks the master's subscription URL as the redirect target.
func SubTarget(s Snapshot) string { return s.MasterSubURL }

// EndpointHandler serves the HA endpoint: /ha/leader and /ha/role for load
// balancers, bots and humans, /ha/peer for the quorum (cluster secret).
func EndpointHandler(code, secret string, self Self, current func() Snapshot) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ha/leader", func(w http.ResponseWriter, _ *http.Request) {
		snap := current()
		if snap.IsMaster() {
			writeJSON(w, http.StatusOK, map[string]any{"master": true, "code": code})
			return
		}
		if snap.MasterPanelURL != "" {
			w.Header().Set(MasterHeader, snap.MasterPanelURL)
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"master": false, "error": "not_master", "role": snap.Role, "reason": snap.Reason,
			"master_code": snap.Master, "master_url": snap.MasterPanelURL,
		})
	})
	mux.HandleFunc("GET /ha/role", func(w http.ResponseWriter, _ *http.Request) {
		snap := current()
		writeJSON(w, http.StatusOK, map[string]any{
			"code": code, "role": snap.Role, "is_master": snap.IsMaster(), "reason": snap.Reason,
			"quorum": snap.Quorum, "seen": snap.Seen, "since": snap.Since,
			"master": snap.Master, "master_url": snap.MasterPanelURL, "master_sub_url": snap.MasterSubURL,
		})
	})
	mux.HandleFunc("GET "+PeerPath, func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if secret == "" || subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "bad cluster secret"})
			return
		}
		writeJSON(w, http.StatusOK, Ping{Code: code, Role: current().Role, PanelURL: self.PanelURL, SubURL: self.SubURL})
	})
	return mux
}

// StandbyHandler answers on the panel and subscription ports of a node that is
// not master: page loads go to the master, everything else gets 409 not_master.
func StandbyHandler(current func() Snapshot, pick func(Snapshot) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		snap := current()
		target := pick(snap)
		if target != "" {
			w.Header().Set(MasterHeader, target)
		}
		isPage := (r.Method == http.MethodGet || r.Method == http.MethodHead) && !strings.Contains(r.URL.Path, "/panel/api/")
		switch {
		case isPage && target != "":
			http.Redirect(w, r, strings.TrimRight(target, "/")+r.URL.RequestURI(), http.StatusTemporaryRedirect)
		case isPage:
			writeJSON(w, http.StatusServiceUnavailable, notMasterBody(snap, ""))
		default:
			// 409, not 5xx: nothing failed, the request reached the wrong node.
			writeJSON(w, http.StatusConflict, notMasterBody(snap, target))
		}
	})
}

func notMasterBody(snap Snapshot, target string) map[string]any {
	return map[string]any{
		"success": false, "error": "not_master",
		"msg":  fmt.Sprintf("this panel is %s (%s); only the master accepts requests", snap.Role, snap.Reason),
		"role": snap.Role, "reason": snap.Reason, "master": snap.Master, "master_url": target,
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// GinGuard makes a panel that lost master answer like a standby at once,
// before the process has finished shutting down.
func GinGuard(current func() Snapshot, pick func(Snapshot) string) gin.HandlerFunc {
	standby := StandbyHandler(current, pick)
	return func(c *gin.Context) {
		if current().IsMaster() {
			c.Next()
			return
		}
		standby.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}
