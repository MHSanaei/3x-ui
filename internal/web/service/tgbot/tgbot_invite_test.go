package tgbot

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"github.com/mymmrac/telego"
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

func mustInvitePayload(t *testing.T, subID string) string {
	t.Helper()
	payload, ok := encodeInvitePayload(subID)
	if !ok {
		t.Fatalf("encodeInvitePayload(%q) refused", subID)
	}
	return payload
}

// newInviteTgbot seeds one unbound client whose subId is sub-invite.
func newInviteTgbot(t *testing.T, email string) (*Tgbot, func(string) int) {
	t.Helper()
	tb, calls := newLinksCallbackTgbot(t, email)
	if err := database.GetDB().Model(&model.Inbound{}).Where("1 = 1").
		Update("settings", `{"clients":[{"id":"6f1d2c3e-8a4b-4c5d-9e6f-7a8b9c0d1e2f","email":"`+email+`","subId":"sub-invite"}]}`).Error; err != nil {
		t.Fatalf("unbind seeded client: %v", err)
	}
	seedClientRecord(t, email, "sub-invite", 0)
	withAdmins(t, 1)
	return tb, calls
}

// Regression test: a subId with URL metacharacters was pasted raw into the link,
// so Telegram truncated it; every legal subId that fits must survive the trip.
func TestInvitePayloadRoundTrip(t *testing.T) {
	for _, subID := range []string{"a1B2c3D4e5F6g7H8", "team#1", "alice&bob", "x?y=z", "کاربر", strings.Repeat("s", 48)} {
		payload, ok := encodeInvitePayload(subID)
		if !ok {
			t.Errorf("encodeInvitePayload(%q) refused", subID)
			continue
		}
		if strings.Trim(payload, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") != "" {
			t.Errorf("payload %q for %q has characters Telegram rejects", payload, subID)
		}
		if got, ok := decodeInvitePayload(payload); !ok || got != subID {
			t.Errorf("round trip of %q = (%q, %v)", subID, got, ok)
		}
	}
	if _, ok := encodeInvitePayload(strings.Repeat("s", 49)); ok {
		t.Error("a subId past 64 payload characters must be refused, not truncated")
	}
	for _, payload := range []string{"", "not base64!", "   "} {
		if _, ok := decodeInvitePayload(payload); ok {
			t.Errorf("decodeInvitePayload(%q) accepted", payload)
		}
	}
}

// Regression test: accounts opening one link at once all read TgID == 0, all bound
// and were all told so, while only the last write held; exactly one may succeed.
func TestConcurrentClaimsBindOnlyOneAccount(t *testing.T) {
	tb, _ := newInviteTgbot(t, "raced@x")
	payload := mustInvitePayload(t, "sub-invite")

	claimants := []int64{8101, 8102, 8103, 8104, 8105, 8106}
	outcomes := make([]inviteOutcome, len(claimants))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, id := range claimants {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			outcomes[i] = tb.claimInvite(id, id, payload)
		}()
	}
	close(start)
	wg.Wait()

	told, holders := 0, 0
	for i, id := range claimants {
		if outcomes[i] == inviteBindable {
			told++
		}
		if tb.levelOf(id) == levelClient {
			holders++
		}
	}
	if told != 1 || holders != 1 {
		t.Errorf("%d accounts told they bound, %d holding the client; want 1 and 1", told, holders)
	}
}

// A stranger opening a valid invite link must come out of it a client.
func TestClaimInvitePromotesStrangerToClient(t *testing.T) {
	const email = "invitee@x"
	tb, calls := newInviteTgbot(t, email)

	const newcomer = int64(8080)
	if got := tb.levelOf(newcomer); got != levelStranger {
		t.Fatalf("levelOf before claim = %d, want stranger", got)
	}

	tb.claimInvite(newcomer, newcomer, mustInvitePayload(t, "sub-invite"))

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

// Guessing a subId must stay slow: past five attempts an hour an account is
// refused, and admins are told once per window rather than once per attempt.
func TestInviteAttemptLimit(t *testing.T) {
	_, calls := newLinksCallbackTgbot(t, ownerMail)
	withAdmins(t, 1, 2)
	tb := &Tgbot{}

	now := time.Unix(1_700_000_000, 0)
	origNow, origBy := inviteAttemptsNow, inviteAttemptsBy
	inviteAttemptsNow = func() time.Time { return now }
	inviteAttemptsBy = map[int64]*inviteAttempts{}
	t.Cleanup(func() { inviteAttemptsNow, inviteAttemptsBy = origNow, origBy })

	guesser := &telego.User{ID: 6666, FirstName: "<b>x</b>"}
	for i := 1; i <= inviteAttemptLimit; i++ {
		if !tb.allowInviteAttempt(guesser) {
			t.Fatalf("attempt %d refused, want allowed", i)
		}
	}
	for range 3 {
		if tb.allowInviteAttempt(guesser) {
			t.Fatal("attempt past the limit allowed")
		}
	}
	if n := calls("sendMessage"); n != 2 {
		t.Errorf("sendMessage calls = %d, want 2: one notice per admin, once per window", n)
	}
	if !tb.allowInviteAttempt(&telego.User{ID: 7777}) {
		t.Error("another account was refused by the guesser's limit")
	}

	now = now.Add(inviteAttemptWindow)
	if !tb.allowInviteAttempt(guesser) {
		t.Error("attempt after the window refused, want allowed")
	}
}

func TestTgUserMentionEscapesName(t *testing.T) {
	got := tgUserMention(&telego.User{ID: 42, FirstName: "<b>Eve</b>", Username: "eve"})
	want := `<a href="tg://user?id=42">&lt;b&gt;Eve&lt;/b&gt;</a> @eve`
	if got != want {
		t.Errorf("tgUserMention = %q, want %q", got, want)
	}
}
