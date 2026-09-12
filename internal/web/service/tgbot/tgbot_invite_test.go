package tgbot

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func initInviteDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func seedClient(t *testing.T, email, subID string, tgID int64) {
	t.Helper()
	rec := &model.ClientRecord{Email: email, SubID: subID, TgID: tgID, Enable: true}
	if err := database.GetDB().Create(rec).Error; err != nil {
		t.Fatalf("seed client %s: %v", email, err)
	}
}

// A SubID doubles as the invite token, so binding is first-claim-wins: the owner
// re-tapping is idempotent, and a client held by someone else is never reassigned.
func TestResolveInviteToken(t *testing.T) {
	initInviteDB(t)
	seedClient(t, "unclaimed@x", "subfree0000000001", 0)
	seedClient(t, "owned@x", "subowned000000002", 5150)

	tg := &Tgbot{}

	tests := []struct {
		name  string
		token string
		from  int64
		want  inviteOutcome
		email string
	}{
		{"unclaimed binds", "subfree0000000001", 777, inviteBindable, "unclaimed@x"},
		{"owner is idempotent", "subowned000000002", 5150, inviteAlreadyOwned, "owned@x"},
		{"stranger is refused", "subowned000000002", 999, inviteTaken, "owned@x"},
		{"unknown token", "nosuchtoken000000", 777, inviteInvalid, ""},
		{"empty token", "", 777, inviteInvalid, ""},
		{"blank token", "   ", 777, inviteInvalid, ""},
		{"missing sender id", "subfree0000000001", 0, inviteInvalid, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, recs := tg.resolveInviteToken(tc.token, tc.from)
			if got != tc.want {
				t.Fatalf("outcome = %v, want %v", got, tc.want)
			}
			if tc.email == "" {
				if len(recs) != 0 {
					t.Fatalf("records = %+v, want none", recs)
				}
				return
			}
			if len(recs) != 1 {
				t.Fatalf("resolved %d records, want exactly 1", len(recs))
			}
			if recs[0].Email != tc.email {
				t.Fatalf("record.Email = %q, want %q", recs[0].Email, tc.email)
			}
		})
	}
}

// Whitespace around a token is common when a link is copied by hand; it must
// still resolve rather than being reported as an invalid invite.
func TestResolveInviteTokenTrimsWhitespace(t *testing.T) {
	initInviteDB(t)
	seedClient(t, "trim@x", "subtrim0000000003", 0)

	tg := &Tgbot{}
	got, recs := tg.resolveInviteToken("  subtrim0000000003\n", 42)
	if got != inviteBindable {
		t.Fatalf("outcome = %v, want inviteBindable", got)
	}
	if len(recs) != 1 || recs[0].Email != "trim@x" {
		t.Fatalf("records = %+v, want trim@x", recs)
	}
}

// A subscription can span several clients, so a token mapping to more than one must
// bind every unbound part — otherwise the customer silently loses the rest.
func TestClaimInviteBindsEveryClientSharingSubID(t *testing.T) {
	initInviteDB(t)
	const shared = "subshared00000003"
	seedClient(t, "multi-a@x", shared, 0)
	seedClient(t, "multi-b@x", shared, 0)
	seedClient(t, "multi-c@x", shared, 0)

	tg := &Tgbot{}
	outcome, records := tg.resolveInviteToken(shared, 7000)
	if outcome != inviteBindable {
		t.Fatalf("outcome = %v, want inviteBindable", outcome)
	}
	if len(records) != 3 {
		t.Fatalf("resolved %d records, want all 3 sharing the subId", len(records))
	}
}

// A token part-owned by a third party must not be claimable: one stranger
// holding a slice of a shared subscription blocks the whole token.
func TestResolveInviteTokenRefusesPartiallyTakenSubID(t *testing.T) {
	initInviteDB(t)
	const shared = "subpartial0000004"
	seedClient(t, "part-free@x", shared, 0)
	seedClient(t, "part-held@x", shared, 9999)

	tg := &Tgbot{}
	if outcome, _ := tg.resolveInviteToken(shared, 7001); outcome != inviteTaken {
		t.Fatalf("outcome = %v, want inviteTaken", outcome)
	}
}

// The vague public reply is right for a stranger and useless for an admin, who
// needs to know which account is holding the token.
func TestInviteDiagnosis(t *testing.T) {
	initInviteDB(t)
	seedClient(t, "diag@x", "subdiag0000000005", 4242)
	tg := &Tgbot{}

	// Without a localizer I18nBot echoes the key and drops its parameters, so
	// the holder detail is asserted on the helper that builds it.
	_, records := tg.resolveInviteToken("subdiag0000000005", 7002)
	holders := inviteHolders(records)
	if !strings.Contains(holders, "diag@x") || !strings.Contains(holders, "4242") {
		t.Fatalf("holders %q should name the client and the holding Telegram id", holders)
	}
	if key := tg.inviteDiagnosis("subdiag0000000005", records); key != "tgbot.messages.inviteDiagTaken" {
		t.Fatalf("diagnosis key = %q, want the already-bound key", key)
	}

	_, none := tg.resolveInviteToken("nosuchtoken000006", 7002)
	if inviteHolders(none) != "" {
		t.Fatal("an unknown token has no holders")
	}
	if key := tg.inviteDiagnosis("nosuchtoken000006", none); key != "tgbot.messages.inviteDiagUnknown" {
		t.Fatalf("diagnosis key = %q, want the unknown-token key", key)
	}
}

// A second subscription is allowed only up to the ceiling: without one a customer
// could collect other people's configs by collecting their invite links.
func TestBindingHeadroom(t *testing.T) {
	initInviteDB(t)
	seedClient(t, "held-a@x", "subheld0000000007", 8100)
	seedClient(t, "held-b@x", "subheld0000000107", 8100)
	seedClient(t, "offered@x", "suboffer000000008", 0)

	tg := &Tgbot{}
	_, offered := tg.resolveInviteToken("suboffer000000008", 8100)

	tests := []struct {
		name  string
		limit int
		tgID  int64
		want  bool
	}{
		{"a holder may take a second config", 5, 8100, true},
		{"a limit of one restores the old rule", 1, 8100, false},
		{"a full account is refused", 2, 8100, false},
		{"unlimited never refuses", 0, 8100, true},
		{"an account holding nothing may claim", 1, 8101, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tg.settingService.SetTgBotMaxBindings(tc.limit); err != nil {
				t.Fatalf("SetTgBotMaxBindings: %v", err)
			}
			allowed, limit := tg.bindingHeadroom(tc.tgID, offered)
			if allowed != tc.want {
				t.Fatalf("bindingHeadroom = %v, want %v", allowed, tc.want)
			}
			if limit != tc.limit {
				t.Fatalf("reported limit = %d, want %d", limit, tc.limit)
			}
		})
	}
}

// Re-tapping the link for a subscription the caller already partly holds must
// still complete the binding rather than being read as a second config.
func TestBindingHeadroomIgnoresTheTokenBeingClaimed(t *testing.T) {
	initInviteDB(t)
	const shared = "subresume00000009"
	seedClient(t, "resume-a@x", shared, 8200)
	seedClient(t, "resume-b@x", shared, 0)

	tg := &Tgbot{}
	if err := tg.settingService.SetTgBotMaxBindings(1); err != nil {
		t.Fatalf("SetTgBotMaxBindings: %v", err)
	}
	_, records := tg.resolveInviteToken(shared, 8200)
	if allowed, _ := tg.bindingHeadroom(8200, records); !allowed {
		t.Fatal("records behind the claimed token must not count against the ceiling")
	}
}

// The ceiling counts subscriptions, not client records: counting the parts of one
// multi-inbound subscription would refuse a household long before the limit.
func TestBindingHeadroomCountsSubscriptionsNotRecords(t *testing.T) {
	initInviteDB(t)
	const spread = "subspread00000010"
	seedClient(t, "spread-a@x", spread, 8300)
	seedClient(t, "spread-b@x", spread, 8300)
	seedClient(t, "spread-c@x", spread, 8300)
	seedClient(t, "next@x", "subnext0000000011", 0)

	tg := &Tgbot{}
	if err := tg.settingService.SetTgBotMaxBindings(2); err != nil {
		t.Fatalf("SetTgBotMaxBindings: %v", err)
	}
	_, offered := tg.resolveInviteToken("subnext0000000011", 8300)
	if allowed, _ := tg.bindingHeadroom(8300, offered); !allowed {
		t.Fatal("three inbounds of one subscription must count as one config")
	}
}

// The admin notice means "a new customer just came online", so a re-tap of a
// link by the account that already holds it must not count as an arrival.
func TestFreshBindings(t *testing.T) {
	const holder = int64(7000)

	tests := []struct {
		name    string
		records []*model.ClientRecord
		want    []string
	}{
		{
			name:    "an unclaimed client is new",
			records: []*model.ClientRecord{{Email: "a@x", TgID: 0}},
			want:    []string{"a@x"},
		},
		{
			name:    "every unclaimed client behind the token is new",
			records: []*model.ClientRecord{{Email: "a@x", TgID: 0}, {Email: "b@x", TgID: 0}},
			want:    []string{"a@x", "b@x"},
		},
		{
			name:    "a re-tap by the holder is not new",
			records: []*model.ClientRecord{{Email: "a@x", TgID: holder}},
			want:    nil,
		},
		{
			name:    "a client held by someone else is not new",
			records: []*model.ClientRecord{{Email: "a@x", TgID: 999}},
			want:    nil,
		},
		{
			name:    "a partly claimed token reports only what binds",
			records: []*model.ClientRecord{{Email: "a@x", TgID: holder}, {Email: "b@x", TgID: 0}},
			want:    []string{"b@x"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := freshBindings(tc.records)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("freshBindings = %v, want %v", got, tc.want)
			}
		})
	}
}

// The notice exists so an admin can reach the person who just arrived, and it
// carries user-controlled text into an HTML message.
func TestNewClientNotice(t *testing.T) {
	tg := &Tgbot{}

	tests := []struct {
		name      string
		firstName string
		username  string
		want      []string
		absent    []string
	}{
		{
			name:      "links to the user by id",
			firstName: "Amy",
			want:      []string{`tg://user?id=7000`, "Amy", "7000"},
		},
		{
			name:      "includes the @username when there is one",
			firstName: "Amy",
			username:  "amyx",
			want:      []string{"@amyx"},
		},
		{
			name:      "omits the @ when there is no username",
			firstName: "Amy",
			want:      []string{"Amy"},
			absent:    []string{"(@"},
		},
		{
			name:      "escapes markup in the display name",
			firstName: `<b>bold</b>`,
			want:      []string{"&lt;b&gt;bold&lt;/b&gt;"},
			absent:    []string{"<b>bold</b>"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			notice := tg.newClientNotice(7000, tc.firstName, tc.username, []string{"a@x"}, 1)
			for _, want := range tc.want {
				if !strings.Contains(notice, want) {
					t.Fatalf("notice is missing %q: %q", want, notice)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(notice, absent) {
					t.Fatalf("notice leaks %q: %q", absent, notice)
				}
			}
		})
	}
}

// An admin must see on the arrival notice when a customer holds several configs,
// while a first arrival must read exactly as it did before.
func TestNewClientNoticeNamesTheHoldingCount(t *testing.T) {
	initLangDB(t)
	tg := new(Tgbot)

	if notice := tg.newClientNotice(7000, "Amy", "", []string{"a@x"}, 1); strings.Contains(notice, "newClientHolding") {
		t.Fatalf("a first arrival must not carry a holding count: %q", notice)
	}
	if notice := tg.newClientNotice(7000, "Amy", "", []string{"b@x"}, 3); !strings.Contains(notice, "newClientHolding") {
		t.Fatalf("a repeat holder must carry a holding count: %q", notice)
	}
}
