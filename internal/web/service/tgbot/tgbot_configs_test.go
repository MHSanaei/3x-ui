package tgbot

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// customerLocalizer renders the templates the header and the "nothing bound"
// reply use. Without it I18n hands back the bare key, and an assertion that a
// header "names the config" would pass against the key itself.
func customerLocalizer(t *testing.T) {
	t.Helper()
	bundle := i18n.NewBundle(language.MustParse("en-US"))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	_ = bundle.AddMessages(language.MustParse("en-US"),
		&i18n.Message{ID: "tgbot.messages.clientHeader", Other: "EMAIL={{ .Email }}|EXP={{ .Expiry }}|LEFT={{ .Remaining }}"},
		&i18n.Message{ID: "tgbot.messages.unlimited", Other: "Unlimited"},
		&i18n.Message{ID: "tgbot.messages.notStarted", Other: "not started"},
		&i18n.Message{ID: "tgbot.messages.noBoundClient", Other: "No config is linked yet."},
		&i18n.Message{ID: "tgbot.messages.noBoundClientAdmin", Other: "You have not bound a config."},
		&i18n.Message{ID: "tgbot.messages.customerMenu", Other: "What do you need?"},
		&i18n.Message{ID: "tgbot.messages.guideChoose", Other: "Pick a device."},
		&i18n.Message{ID: "tgbot.buttons.backToMenu", Other: "Back"},
		&i18n.Message{ID: "tgbot.buttons.getAllLinks", Other: "All links"},
		&i18n.Message{ID: "tgbot.buttons.getOneConfig", Other: "One link"},
		&i18n.Message{ID: "tgbot.buttons.subscriptionUrl", Other: "Subscription URL"},
		&i18n.Message{ID: "tgbot.buttons.qrCodes", Other: "QR codes"},
		&i18n.Message{ID: "tgbot.buttons.setupGuide", Other: "Setup guide"},
		&i18n.Message{ID: "tgbot.buttons.customerMenu", Other: "My configs"},
		&i18n.Message{ID: "tgbot.messages.configsChoose", Other: "Choose how:"},
		&i18n.Message{ID: "tgbot.buttons.guideIos", Other: "iPhone"},
		&i18n.Message{ID: "tgbot.buttons.guideAndroid", Other: "Android"},
		&i18n.Message{ID: "tgbot.buttons.guideWindows", Other: "Windows"},
		&i18n.Message{ID: "tgbot.buttons.guideMacos", Other: "macOS"},
		&i18n.Message{ID: "tgbot.buttons.guideLinux", Other: "Linux"},
		&i18n.Message{ID: "tgbot.messages.guideIos", Other: "iPhone guide."},
		&i18n.Message{ID: "tgbot.messages.guideAndroid", Other: "Android guide."},
		&i18n.Message{ID: "tgbot.messages.guideWindows", Other: "Windows guide."},
		&i18n.Message{ID: "tgbot.messages.guideMacos", Other: "macOS guide."},
		&i18n.Message{ID: "tgbot.messages.guideLinux", Other: "Linux guide."},
		&i18n.Message{ID: "tgbot.messages.chooseOneConfig", Other: "Which one?"},
		&i18n.Message{ID: "tgbot.noResult", Other: "No result"},
		&i18n.Message{ID: "tgbot.answers.errorOperation", Other: "Error."},
	)
	orig := locale.LocalizerBot
	t.Cleanup(func() { locale.LocalizerBot = orig })
	locale.LocalizerBot = i18n.NewLocalizer(bundle, "en-US")
}

// A subscription link names the client after "#", so the picker shows that
// remark rather than several identical "vless://" rows.
func TestClientLinkLabel(t *testing.T) {
	tests := []struct {
		name string
		link string
		want string
	}{
		{name: "remark from fragment", link: "vless://uuid@host:443?x=1#Home%20Server", want: "1. Home Server"},
		{name: "plain remark", link: "vmess://uuid@host#Office", want: "1. Office"},
		{name: "no fragment falls back to scheme", link: "trojan://uuid@host:443", want: "1. TROJAN"},
		{name: "empty fragment", link: "vless://uuid@host#", want: "1. VLESS"},
		{name: "email-like fragment", link: "vless://u@h#a@b", want: "1. a@b"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := clientLinkLabel(tc.link, 0); got != tc.want {
				t.Fatalf("clientLinkLabel(%q, 0) = %q, want %q", tc.link, got, tc.want)
			}
		})
	}

	// The label counts from one so the picker reads 1., 2., 3. in the order
	// the buttons are stacked.
	t.Run("index is one-based", func(t *testing.T) {
		if got := clientLinkLabel("vless://u@h#B", 1); got != "2. B" {
			t.Fatalf("clientLinkLabel at index 1 = %q, want %q", got, "2. B")
		}
	})
}

// The index travels in callback data, so a stale or hostile one must be
// refused rather than reading past the end of the link slice.
func TestSendOneLinkRefusesAnOutOfRangeIndex(t *testing.T) {
	tb, calls := newLinksCallbackTgbot(t, ownerMail)

	for _, index := range []int{-1, 99} {
		tb.sendOneLink(7001, ownerMail, index)
	}

	if n := calls("sendMessage"); n == 0 {
		t.Error("an out-of-range index was answered with silence instead of an error")
	}
}

// Regression test: a customer may only act on a config bound to their own
// Telegram id. Without the check a foreign email in callback data is served,
// since that data outlives the message it was sent with.
func TestCustomerCallbackRefusesAForeignClient(t *testing.T) {
	tb, calls := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)

	tapClientLinks(t, tb, ownerTgID, "client_one_link someone-else@x")

	if n := calls("sendMessage"); n != 0 {
		t.Errorf("sendMessage calls = %d, want 0: a customer received a foreign client's picker", n)
	}
	if n := calls("answerCallbackQuery"); n != 1 {
		t.Errorf("answerCallbackQuery calls = %d, want 1: the refused tap must still be answered", n)
	}
}

// The same guard must not lock the owner out of their own config.
func TestCustomerCallbackServesOwnClient(t *testing.T) {
	tb, calls := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)

	tapClientLinks(t, tb, ownerTgID, "client_one_link "+ownerMail)

	if n := calls("answerCallbackQuery"); n != 1 {
		t.Errorf("answerCallbackQuery calls = %d, want 1: an allowed tap must be answered", n)
	}
}

// link_one carries a second attacker-chosen field, so the index must not be
// what the ownership check compares.
func TestIndexedLinkCallbackIgnoresASwappedEmail(t *testing.T) {
	tb, calls := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)

	tapClientLinks(t, tb, ownerTgID, "link_one someone-else@x 0")

	if n := calls("sendMessage"); n != 0 {
		t.Errorf("sendMessage calls = %d, want 0: link_one served a foreign client's link", n)
	}
}

// The configs step must name the config before offering buttons that act on it,
// so a customer holding several is never handed one person's links.
func TestConfigsMenuNamesTheConfigFirst(t *testing.T) {
	tb, calls := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)
	runningBot(t)

	tb.configsMenu(7001, ownerMail)

	if n := calls("sendMessage"); n != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", n)
	}
}

// Every button on the configs step must pass the customer gate, since each one
// carries the email this branch resolved.
func TestConfigsKeyboardButtonsPassTheGate(t *testing.T) {
	for _, data := range []string{
		"client_individual_links " + ownerMail,
		"client_one_link " + ownerMail,
		"client_qr_links " + ownerMail,
		"client_menu",
	} {
		if !isClientSelfCallback(data) {
			t.Errorf("isClientSelfCallback(%q) = false, want true", data)
		}
	}
}

// Admin and customer buttons are encoded by the same encodeQuery, so the
// customer verb set must not swallow the three callbacks an admin already had.
func TestSplitClientSelfActionLeavesAdminCallbacksAlone(t *testing.T) {
	for _, data := range []string{
		"client_one_link " + ownerMail,
		"link_one " + ownerMail + " 0",
	} {
		if _, _, ok := splitClientSelfAction(data); !ok {
			t.Errorf("splitClientSelfAction(%q) did not match", data)
		}
	}
	// main's allowlist still has to accept the bare, email-less form of the
	// three it already serves: the picker answers those by listing the
	// caller's own configs.
	for _, data := range []string{"client_sub_links", "client_individual_links", "client_qr_links"} {
		if !isClientSelfCallback(data) {
			t.Errorf("isClientSelfCallback(%q) = false, want true", data)
		}
		if _, _, ok := splitClientSelfAction(data); ok {
			t.Errorf("splitClientSelfAction(%q) claimed a callback this part does not add", data)
		}
		// The bare and email-bearing forms of main's verbs must both pass the gate.
		if !isClientSelfCallback(data + " " + ownerMail) {
			t.Errorf("isClientSelfCallback(%q + email) = false, want true", data)
		}
	}
}

// ownsClient is the check itself; the callback test above only proves it is
// wired up, so pin its three outcomes directly.
func TestOwnsClient(t *testing.T) {
	tb, _ := newCustomerTgbot(t, ownerMail)

	if !tb.ownsClient(ownerTgID, ownerMail) {
		t.Error("the owner does not own their own client")
	}
	if tb.ownsClient(ownerTgID, "someone-else@x") {
		t.Error("a foreign client was treated as the caller's own")
	}
	if tb.ownsClient(9999, ownerMail) {
		t.Error("an unbound account was treated as the owner")
	}
}

// newCustomerTgbot seeds one inbound plus the clients-table row that binds it
// to a Telegram id. The binding check reads the clients table, while the link
// senders read traffic, so both have to exist before a customer tap can be
// exercised end to end.
func newCustomerTgbot(t *testing.T, email string) (*Tgbot, func(string) int) {
	t.Helper()
	tb, calls := newLinksCallbackTgbot(t, email)
	seedClientRecord(t, email, "sub-owned", ownerTgID)
	if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", email).
		Updates(map[string]any{"total": 1024 * 1024 * 1024, "up": 1024 * 1024 * 10, "down": 1024 * 1024 * 20}).Error; err != nil {
		t.Fatalf("cap traffic: %v", err)
	}
	if err := database.GetDB().Model(&model.Inbound{}).Where("remark = ?", "in").
		Update("settings", `{"clients":[{"email":"`+email+`","tgId":4242,"subId":"sub-owned"}]}`).Error; err != nil {
		t.Fatalf("seed inbound settings: %v", err)
	}
	return tb, calls
}

// A link page must say which config it is showing, or a customer holding
// several cannot tell them apart.
func TestClientHeaderNamesTheConfig(t *testing.T) {
	tb, _ := newCustomerTgbot(t, ownerMail)
	customerLocalizer(t)

	header := tb.clientHeader(ownerMail)
	if header == "" {
		t.Fatal("clientHeader returned nothing for a seeded client")
	}
	if !strings.Contains(header, "EMAIL="+ownerMail) {
		t.Fatalf("header does not name the config: %q", header)
	}
	if !strings.Contains(header, "LEFT=") {
		t.Fatalf("header does not report the remaining quota: %q", header)
	}
}

// A client with no traffic record at all must not produce a broken page; the
// header is dropped and the links still go out.
func TestClientHeaderSurvivesAMissingClient(t *testing.T) {
	tb, _ := newLinksCallbackTgbot(t, ownerMail)
	customerLocalizer(t)
	if got := tb.clientHeader("nobody@x"); got != "" {
		t.Fatalf("clientHeader for an unknown client = %q, want empty", got)
	}
}

// An uncapped config has no quota, which must read as unlimited rather than as
// zero remaining: the latter tells a customer they are cut off.
func TestClientRemainingLabel(t *testing.T) {
	tb := &Tgbot{}
	customerLocalizer(t)
	unlimited := tb.I18nBot("tgbot.messages.unlimited")

	if got := clientRemainingLabel(tb, 30, 0); got != unlimited {
		t.Errorf("uncapped remaining = %q, want %q", got, unlimited)
	}
	if got := clientRemainingLabel(tb, 30, 100); got == "" || got == unlimited {
		t.Errorf("a capped config reported %q", got)
	}
	// Spending past the total must not render as a negative amount.
	if got := clientRemainingLabel(tb, 500, 100); got == "" {
		t.Error("an over-quota config reported nothing")
	}
}

func TestClientExpiryLabelCoversEveryCase(t *testing.T) {
	tb := &Tgbot{}
	customerLocalizer(t)
	unlimited := tb.I18nBot("tgbot.messages.unlimited")
	notStarted := tb.I18nBot("tgbot.messages.notStarted")

	if got := clientExpiryLabel(tb, 0); got != unlimited {
		t.Errorf("expiry 0 = %q, want %q", got, unlimited)
	}
	if got := clientExpiryLabel(tb, -1); got != notStarted {
		t.Errorf("expiry -1 = %q, want %q", got, notStarted)
	}
	if got := clientExpiryLabel(tb, 1700000000000); got == unlimited || got == notStarted {
		t.Errorf("a dated config reported %q", got)
	}
}

// A customer with exactly one config should not be asked to choose from a list
// of one, and a customer with none must be told how to bind rather than shown a
// menu whose every button would refuse them.
func TestClientEmailPicker(t *testing.T) {
	t.Run("one config is used without asking", func(t *testing.T) {
		tb, calls := newCustomerTgbot(t, ownerMail)
		customerLocalizer(t)

		emails := tb.clientEmailPicker(7001, ownerTgID, "client_one_link", false)

		if len(emails) != 1 || emails[0] != ownerMail {
			t.Fatalf("emails = %v, want just the owner's", emails)
		}
		if n := calls("sendMessage"); n != 1 {
			t.Errorf("sendMessage = %d, want 1: the single config must be served without asking", n)
		}
		if n := calls("answerCallbackQuery"); n != 0 {
			t.Errorf("answerCallbackQuery = %d, want 0: the picker asks no question", n)
		}
	})

	t.Run("nothing bound is explained", func(t *testing.T) {
		tb, _ := newLinksCallbackTgbot(t, "someone-else@x")
		customerLocalizer(t)

		if emails := tb.clientEmailPicker(7001, 9999, "client_one_link", false); emails != nil {
			t.Fatalf("emails = %v, want nil for an unbound account", emails)
		}
	})
}
