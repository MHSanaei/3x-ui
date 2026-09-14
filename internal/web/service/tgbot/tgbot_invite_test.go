package tgbot

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func seedClientRecord(t *testing.T, email, subID string, tgID int64) {
	t.Helper()
	rec := &model.ClientRecord{Email: email, SubID: subID, TgID: tgID, Enable: true}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("seed client %s: %v", email, err)
	}
}

// Binding is first-claim-wins: the owner re-tapping is idempotent, and no part of
// a subscription held by someone else is ever reassigned.
func TestResolveInviteToken(t *testing.T) {
	tb, _ := newLinksCallbackTgbot(t, ownerMail)
	seedClientRecord(t, "free@x", "sub-free", 0)
	seedClientRecord(t, "held@x", "sub-held", 5150)
	seedClientRecord(t, "shared-a@x", "sub-shared", 0)
	seedClientRecord(t, "shared-b@x", "sub-shared", 0)
	seedClientRecord(t, "part-mine@x", "sub-part-mine", 7000)
	seedClientRecord(t, "part-free@x", "sub-part-mine", 0)
	seedClientRecord(t, "part-free2@x", "sub-part-held", 0)
	seedClientRecord(t, "part-held@x", "sub-part-held", 9999)

	cases := []struct {
		name    string
		token   string
		from    int64
		want    inviteOutcome
		records int
	}{
		{"unclaimed binds", "sub-free", 7000, inviteBindable, 1},
		{"token is trimmed", "  sub-free\n", 7000, inviteBindable, 1},
		{"owner is idempotent", "sub-held", 5150, inviteAlreadyOwned, 1},
		{"someone else's is refused", "sub-held", 7000, inviteTaken, 1},
		{"shared subscription binds whole", "sub-shared", 7000, inviteBindable, 2},
		{"finishing a partly owned one binds", "sub-part-mine", 7000, inviteBindable, 2},
		{"partly held by another is refused", "sub-part-held", 7000, inviteTaken, 2},
		{"unknown token", "sub-nope", 7000, inviteInvalid, 0},
		{"empty token", "", 7000, inviteInvalid, 0},
		{"missing sender", "sub-free", 0, inviteInvalid, 0},
	}
	for _, c := range cases {
		got, records := tb.resolveInviteToken(c.token, c.from)
		if got != c.want || len(records) != c.records {
			t.Errorf("%s: got (%d, %d records), want (%d, %d records)", c.name, got, len(records), c.want, c.records)
		}
	}
}

// A stranger opening a valid invite link must come out of it a client.
func TestClaimInvitePromotesStrangerToClient(t *testing.T) {
	const email = "invitee@x"
	tb, calls := newLinksCallbackTgbot(t, email)
	if err := database.GetDB().Model(&model.Inbound{}).Where("1 = 1").
		Update("settings", `{"clients":[{"id":"6f1d2c3e-8a4b-4c5d-9e6f-7a8b9c0d1e2f","email":"`+email+`","subId":"sub-invite"}]}`).Error; err != nil {
		t.Fatalf("unbind seeded client: %v", err)
	}
	seedClientRecord(t, email, "sub-invite", 0)
	withAdmins(t, 1)

	const newcomer = int64(8080)
	if got := tb.levelOf(newcomer); got != levelStranger {
		t.Fatalf("levelOf before claim = %d, want stranger", got)
	}

	tb.claimInvite(newcomer, newcomer, "sub-invite")

	if got := tb.levelOf(newcomer); got != levelClient {
		t.Errorf("levelOf after claim = %d, want client", got)
	}
	if n := calls("sendMessage"); n != 1 {
		t.Errorf("sendMessage calls = %d, want 1 confirmation", n)
	}
	if outcome, _ := tb.resolveInviteToken("sub-invite", 9999); outcome != inviteTaken {
		t.Errorf("second claimant outcome = %d, want taken", outcome)
	}
}
