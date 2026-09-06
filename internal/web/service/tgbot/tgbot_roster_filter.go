package tgbot

import (
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	rosterFilterAll       = "all"
	rosterFilterExpiring  = "expiring"
	rosterFilterExhausted = "exhausted"
	rosterFilterDisabled  = "disabled"
	rosterFilterUnbound   = "unbound"
)

// A filter offered in the roster, paired with the predicate it applies so a
// button cannot be listed without one.
type rosterFilter struct {
	name     string
	labelKey string
	keep     func(*service.ClientWithAttachments, time.Time) bool
}

var rosterFilters = []rosterFilter{
	{
		name: rosterFilterAll, labelKey: "tgbot.buttons.filterAll",
		keep: func(*service.ClientWithAttachments, time.Time) bool { return true },
	},
	// Expiring means the ladder's own window, so "about to lapse" means the
	// same thing in the roster as it does in the reminders.
	{
		name: rosterFilterExpiring, labelKey: "tgbot.buttons.filterExpiring",
		keep: func(c *service.ClientWithAttachments, now time.Time) bool {
			return expiryRung(c.ExpiryTime, now) != rungNone
		},
	},
	{
		name: rosterFilterExhausted, labelKey: "tgbot.buttons.filterExhausted",
		keep: func(c *service.ClientWithAttachments, _ time.Time) bool {
			return quotaRungOf(clientUsage(c), c.TotalGB) == quotaNinetyFive
		},
	},
	{
		name: rosterFilterDisabled, labelKey: "tgbot.buttons.filterDisabled",
		keep: func(c *service.ClientWithAttachments, _ time.Time) bool { return !c.Enable },
	},
	{
		name: rosterFilterUnbound, labelKey: "tgbot.buttons.filterUnbound",
		keep: func(c *service.ClientWithAttachments, _ time.Time) bool { return c.TgID == 0 },
	},
}

func clientUsage(c *service.ClientWithAttachments) int64 {
	if c.Traffic == nil {
		return 0
	}
	return c.Traffic.Up + c.Traffic.Down
}

// An unknown name shows everything rather than nothing: callback data is not
// trusted, and an empty roster reads as "no clients" rather than "bad filter".
func filterClients(clients []service.ClientWithAttachments, name string, now time.Time) []service.ClientWithAttachments {
	keep := func(*service.ClientWithAttachments, time.Time) bool { return true }
	for _, filter := range rosterFilters {
		if filter.name == name {
			keep = filter.keep
			break
		}
	}

	kept := make([]service.ClientWithAttachments, 0, len(clients))
	for i := range clients {
		if keep(&clients[i], now) {
			kept = append(kept, clients[i])
		}
	}
	return kept
}

// Email and comment match on a substring; a Telegram id must match exactly, or
// searching for a short number would sweep in every id containing it.
func searchClients(clients []service.ClientWithAttachments, query string) []service.ClientWithAttachments {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	needle := strings.ToLower(query)

	found := make([]service.ClientWithAttachments, 0, len(clients))
	for i := range clients {
		client := &clients[i]
		if strings.Contains(strings.ToLower(client.Email), needle) ||
			(client.Comment != "" && strings.Contains(strings.ToLower(client.Comment), needle)) ||
			(client.TgID != 0 && strconv.FormatInt(client.TgID, 10) == query) {
			found = append(found, *client)
		}
	}
	return found
}

func (t *Tgbot) rosterFilterKeyboard(active string) *telego.InlineKeyboardMarkup {
	buttons := make([]telego.InlineKeyboardButton, 0, len(rosterFilters))
	for _, filter := range rosterFilters {
		label := t.I18nBot(filter.labelKey)
		if filter.name == active {
			label = languageMark + label
		}
		buttons = append(buttons, tu.InlineKeyboardButton(label).
			WithCallbackData(t.encodeQuery("roster_filter "+filter.name)))
	}
	rows := tu.InlineKeyboardCols(3, buttons...)
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.rosterSearch")).WithCallbackData(t.encodeQuery("roster_search")),
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.backToAdminPanel")).WithCallbackData(t.encodeQuery("admin_clients")),
	))
	return tu.InlineKeyboardGrid(rows)
}
