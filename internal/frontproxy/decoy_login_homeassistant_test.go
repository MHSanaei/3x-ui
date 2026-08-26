package frontproxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func haStartFlow(t *testing.T, h http.Handler, ip string) haFlowForm {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, reqFrom(http.MethodPost, haLoginFlowPath, ip, `{"client_id":"http://x/","handler":["homeassistant",null],"redirect_uri":"http://x/"}`))
	if w.Code != http.StatusOK {
		t.Fatalf("start flow: status = %d, want 200", w.Code)
	}
	var f haFlowForm
	if err := json.Unmarshal(w.Body.Bytes(), &f); err != nil {
		t.Fatalf("start flow: bad JSON: %v", err)
	}
	if f.Type != "form" || f.StepID != "init" || f.FlowID == "" {
		t.Fatalf("start flow: got %+v, want a fresh init-step form with a flow_id", f)
	}
	if f.Errors != nil {
		t.Fatalf("start flow: errors = %v, want nil on a fresh flow", f.Errors)
	}
	return f
}

func haSubmit(h http.Handler, ip, flowID string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, reqFrom(http.MethodPost, haLoginFlowPath+"/"+flowID, ip, `{"client_id":"http://x/","username":"admin","password":"wrong"}`))
	return w
}

func TestHomeAssistantFlowRejectsWithInvalidAuth(t *testing.T) {
	h := newDecoyHandler(DecoyConfig{Mode: DecoyTemplate, Template: "homeassistant", Seed: "test"})
	ip := "203.0.113.10"
	flow := haStartFlow(t, h, ip)

	w := haSubmit(h, ip, flow.FlowID)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("submit: status = %d, want 400", w.Code)
	}
	var step haFlowForm
	if err := json.Unmarshal(w.Body.Bytes(), &step); err != nil {
		t.Fatalf("submit: bad JSON: %v", err)
	}
	if step.Errors["base"] != "invalid_auth" {
		t.Fatalf("submit: errors = %v, want base=invalid_auth", step.Errors)
	}
}

func TestHomeAssistantBansEveryPathAfterThreshold(t *testing.T) {
	h := newDecoyHandler(DecoyConfig{Mode: DecoyTemplate, Template: "homeassistant", Seed: "test"})
	mock := loginMocks["homeassistant"]
	ip := "203.0.113.11"
	flow := haStartFlow(t, h, ip)

	// The threshold-th failure itself must still answer normally: real HA's
	// ban middleware blocks the *next* request, not the one that crossed it.
	for i := 0; i < mock.Threshold; i++ {
		w := haSubmit(h, ip, flow.FlowID)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: status = %d, want 400", i+1, w.Code)
		}
	}

	// Now every path is forbidden, not just the login endpoint.
	loginW := haSubmit(h, ip, flow.FlowID)
	if loginW.Code != http.StatusForbidden {
		t.Fatalf("post-ban login attempt: status = %d, want 403", loginW.Code)
	}
	pageW := httptest.NewRecorder()
	h.ServeHTTP(pageW, reqFrom(http.MethodGet, "/", ip, ""))
	if pageW.Code != http.StatusForbidden {
		t.Fatalf("post-ban page request: status = %d, want 403", pageW.Code)
	}
	startW := httptest.NewRecorder()
	h.ServeHTTP(startW, reqFrom(http.MethodPost, haLoginFlowPath, ip, `{}`))
	if startW.Code != http.StatusForbidden {
		t.Fatalf("post-ban flow-start request: status = %d, want 403", startW.Code)
	}
}

func TestHomeAssistantBanIsPerSourceIP(t *testing.T) {
	h := newDecoyHandler(DecoyConfig{Mode: DecoyTemplate, Template: "homeassistant", Seed: "test"})
	mock := loginMocks["homeassistant"]
	banned := "203.0.113.12"
	flow := haStartFlow(t, h, banned)
	for i := 0; i < mock.Threshold; i++ {
		haSubmit(h, banned, flow.FlowID)
	}

	other := "203.0.113.13"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, reqFrom(http.MethodGet, "/", other, ""))
	if w.Code != http.StatusOK {
		t.Fatalf("unrelated IP: status = %d, want 200", w.Code)
	}
}
