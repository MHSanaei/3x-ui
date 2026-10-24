package tgbot

import (
	"strings"
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"github.com/mhsanaei/3x-ui/v3/internal/web/locale"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestReviewTelegramUnavailableUptime(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	_ = bundle.AddMessages(language.English, &i18n.Message{ID: "tgbot.messages.serverUpTime", Other: "Uptime: {{.UpTime}} {{.Unit}}\n"}, &i18n.Message{ID: "tgbot.days", Other: "days"})
	oldLocale := locale.LocalizerBot
	locale.LocalizerBot = i18n.NewLocalizer(bundle, "en")
	t.Cleanup(func() { locale.LocalizerBot = oldLocale })
	statusCache.mutex.Lock()
	oldStatus, oldTime := statusCache.data, statusCache.timestamp
	statusCache.mutex.Unlock()
	serverStatsCache.mutex.Lock()
	oldStats, oldStatsTime := serverStatsCache.data, serverStatsCache.timestamp
	serverStatsCache.mutex.Unlock()
	t.Cleanup(func() {
		statusCache.mutex.Lock()
		statusCache.data, statusCache.timestamp = oldStatus, oldTime
		statusCache.mutex.Unlock()
		serverStatsCache.mutex.Lock()
		serverStatsCache.data, serverStatsCache.timestamp = oldStats, oldStatsTime
		serverStatsCache.mutex.Unlock()
	})
	for _, unavailable := range []bool{true, false} {
		t.Run(map[bool]string{true: "unavailable", false: "valid zero"}[unavailable], func(t *testing.T) {
			bot := &Tgbot{}
			status := &service.Status{Loads: []float64{0, 0, 0}}
			if unavailable {
				status.UnavailableMetrics = []string{"uptime"}
			}
			bot.setCachedStatus(status)
			bot.setCachedServerStats("")
			info := bot.prepareServerUsageInfo()
			want := "Uptime: 0 days"
			if unavailable {
				want = "Uptime: —"
			}
			if !strings.Contains(info, want) {
				t.Fatalf("report did not contain %q: %s", want, info)
			}
		})
	}
}
