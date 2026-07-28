package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestNodeCredentialsNeverMarshal(t *testing.T) {
	raw, err := json.Marshal(&model.Node{
		Id:       7,
		Name:     "node",
		ApiToken: "plain-secret-token",
	})
	if err != nil {
		t.Fatalf("marshal node: %v", err)
	}
	out := string(raw)
	if strings.Contains(out, "plain-secret-token") || strings.Contains(out, "apiToken") {
		t.Fatalf("model.Node JSON leaked api token field: %s", out)
	}
}

func TestNodeViewExposesOnlyCredentialPresence(t *testing.T) {
	setupConflictDB(t)

	svc := &NodeService{}
	reqToken := "write-only-secret"
	view, err := svc.CreateFromRequest(&NodeMutationRequest{
		Name:     "node-view",
		Scheme:   "https",
		Address:  "127.0.0.1",
		Port:     2096,
		ApiToken: &reqToken,
		Enable:   true,
	})
	if err != nil {
		t.Fatalf("create from request: %v", err)
	}
	if !view.HasApiToken {
		t.Fatal("create view should report credential presence")
	}

	got, err := svc.GetViewById(view.Id)
	if err != nil {
		t.Fatalf("get view: %v", err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal view: %v", err)
	}
	out := string(raw)
	if !strings.Contains(out, `"hasApiToken":true`) {
		t.Fatalf("view does not report credential presence: %s", out)
	}
	if strings.Contains(out, reqToken) || strings.Contains(out, "apiToken") {
		t.Fatalf("NodeView leaked plaintext or apiToken key: %s", out)
	}
}

func TestNodeCredentialMutationSemantics(t *testing.T) {
	setupConflictDB(t)
	svc := &NodeService{}

	initial := "initial-token"
	view, err := svc.CreateFromRequest(&NodeMutationRequest{
		Name:     "mut",
		Scheme:   "https",
		Address:  "127.0.0.1",
		Port:     2096,
		ApiToken: &initial,
		Enable:   true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	before := rawStoredNodeToken(t, view.Id)
	if before != initial {
		t.Fatalf("stored token = %q, want %q", before, initial)
	}

	if err := svc.UpdateFromRequest(view.Id, &NodeMutationRequest{
		Name:    "mut-renamed",
		Scheme:  "https",
		Address: "127.0.0.1",
		Port:    2096,
		Enable:  true,
	}); err != nil {
		t.Fatalf("keep-token update: %v", err)
	}
	if after := rawStoredNodeToken(t, view.Id); after != before {
		t.Fatalf("omitted token should keep existing token: %q -> %q", before, after)
	}

	blank := " "
	if err := svc.UpdateFromRequest(view.Id, &NodeMutationRequest{
		Name:     "mut-blank",
		Scheme:   "https",
		Address:  "127.0.0.1",
		Port:     2096,
		ApiToken: &blank,
		Enable:   true,
	}); err != nil {
		t.Fatalf("blank apiToken should keep existing token on update: %v", err)
	}
	if afterBlank := rawStoredNodeToken(t, view.Id); afterBlank != before {
		t.Fatalf("blank token should keep existing token: %q -> %q", before, afterBlank)
	}

	next := "next-token"
	if err := svc.UpdateFromRequest(view.Id, &NodeMutationRequest{
		Name:     "mut",
		Scheme:   "https",
		Address:  "127.0.0.1",
		Port:     2096,
		ApiToken: &next,
		Enable:   true,
	}); err != nil {
		t.Fatalf("replace token: %v", err)
	}
	if replaced := rawStoredNodeToken(t, view.Id); replaced != next {
		t.Fatalf("replace token stored %q, want %q", replaced, next)
	}

	if err := svc.UpdateFromRequest(view.Id, &NodeMutationRequest{
		Name:          "mut",
		Scheme:        "https",
		Address:       "127.0.0.1",
		Port:          2096,
		ClearApiToken: true,
		Enable:        true,
	}); err == nil {
		t.Fatal("enabled non-mtls node must not clear apiToken")
	}
	if err := svc.UpdateFromRequest(view.Id, &NodeMutationRequest{
		Name:          "mut",
		Scheme:        "https",
		Address:       "127.0.0.1",
		Port:          2096,
		ClearApiToken: true,
		Enable:        false,
	}); err != nil {
		t.Fatalf("clear disabled token: %v", err)
	}
	if cleared := rawStoredNodeToken(t, view.Id); cleared != "" {
		t.Fatalf("clear token left stored value %q", cleared)
	}

	if _, err := svc.CreateFromRequest(&NodeMutationRequest{
		Name:          "mtls-only",
		Scheme:        "https",
		Address:       "127.0.0.1",
		Port:          2097,
		Enable:        true,
		TlsVerifyMode: "mtls",
	}); err != nil {
		t.Fatalf("mtls create without token: %v", err)
	}
}

func TestNodeUpdateRequiresTokenWhenNoStoredTokenAndMtlsDisabled(t *testing.T) {
	setupConflictDB(t)
	svc := &NodeService{}

	view, err := svc.CreateFromRequest(&NodeMutationRequest{
		Name:          "mtls-empty",
		Scheme:        "https",
		Address:       "127.0.0.1",
		Port:          2098,
		Enable:        true,
		TlsVerifyMode: "mtls",
	})
	if err != nil {
		t.Fatalf("create mtls node: %v", err)
	}
	blank := ""
	if err := svc.UpdateFromRequest(view.Id, &NodeMutationRequest{
		Name:        "mtls-empty",
		Scheme:      "https",
		Address:     "127.0.0.1",
		Port:        2098,
		ApiToken:    &blank,
		Enable:      true,
		BasePath:    "/",
		OutboundTag: "",
	}); err == nil {
		t.Fatal("enabled non-mtls node without stored token must be rejected")
	}
}

func rawStoredNodeToken(t *testing.T, id int) string {
	t.Helper()
	var n model.Node
	if err := database.GetDB().Select("api_token").Where("id = ?", id).First(&n).Error; err != nil {
		t.Fatalf("load raw node token: %v", err)
	}
	return n.ApiToken
}
