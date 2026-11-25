package tgbot

import (
	"errors"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestRenewalPreviewOffersRecoveryAndExplicitSeparateConfirmation(t *testing.T) {
	tb, _ := newScreenTgbot(t, false)
	localizeWithRealBundle(t)
	op := &service.ClientAdminOperation{Token: "new-preview", Owner: adminTgID, Email: "ali", Kind: "renew", Pending: []service.ClientPendingOperation{{Token: "committed-retry", Owner: adminTgID}}}
	tb.screenClientOperation(adminTgID, op)
	screen, ok := tb.screens().get(adminTgID)
	if !ok || !strings.Contains(screen.body, "already committed") {
		t.Fatalf("missing committed-operation warning: %+v", screen)
	}
	foundRetry, foundSeparate := false, false
	for _, row := range screen.rows {
		for _, b := range row {
			if b.CallbackData == "admin_confirm committed-retry" && strings.Contains(b.Text, "Retry") {
				foundRetry = true
			}
			if b.CallbackData == "admin_confirm new-preview" && strings.Contains(b.Text, "separate") {
				foundSeparate = true
			}
		}
	}
	if !foundRetry || !foundSeparate {
		t.Fatalf("recovery or explicit future confirmation missing: %+v", screen.rows)
	}
}

func TestCommittedErrorNeverRequestsAnotherRenewalPreview(t *testing.T) {
	tb, calls := newScreenTgbot(t, false)
	localizeWithRealBundle(t)
	tb.workflowError(adminTgID, errors.Join(service.ErrClientOperationPending, service.ErrClientOperationChanged))
	for _, call := range calls() {
		if strings.Contains(call.text(), "prepare a new") {
			t.Fatalf("committed error suggested reapplying renewal: %s", call.text())
		}
	}
}
