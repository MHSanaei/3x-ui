package tgbot

import (
	"strings"
	"testing"
)

// An unmarked row is the failure this menu exists to prevent: the admin cannot
// tell the switch that silences their own inbox from the customer's.
func TestEveryNotificationDeclaresAnAudience(t *testing.T) {
	for _, notification := range botNotifications {
		if notification.audience != audienceAdmin && notification.audience != audienceCustomer {
			t.Errorf("%s has no audience mark", notification.callback)
		}
	}
}

// Admin notices are listed first so the two audiences read as blocks; a
// customer notice slipped in above one breaks that grouping.
func TestNotificationsAreGroupedAdminFirst(t *testing.T) {
	seenCustomer := false
	for _, notification := range botNotifications {
		if notification.audience == audienceCustomer {
			seenCustomer = true
			continue
		}
		if seenCustomer {
			t.Errorf("%s is listed after a customer notice", notification.callback)
		}
	}
}

// Without a localizer I18nBot echoes the key, so the label is asserted on its
// prefix: the on/off mark, then the audience, then the name.
func TestNotificationToggleLabelCarriesTheAudience(t *testing.T) {
	tg := &Tgbot{}
	on := func(*Tgbot) (bool, error) { return true, nil }

	tests := []struct {
		name         string
		notification botNotification
		want         string
	}{
		{"customer notice", botNotification{labelKey: "k", audience: audienceCustomer, get: on}, "✅ 👤 k"},
		{"admin notice", botNotification{labelKey: "k", audience: audienceAdmin, get: on}, "✅ 👑 k"},
		{"feature switch", botNotification{labelKey: "k", get: on}, "✅ k"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tg.notificationToggleLabel(tc.notification); got != tc.want {
				t.Fatalf("label = %q, want %q", got, tc.want)
			}
		})
	}

	// The feature menu shares the struct, so a stray mark there would be a
	// meaningless crown next to a switch that sends nothing.
	for _, feature := range botFeatures {
		if strings.ContainsAny(tg.notificationToggleLabel(feature), string(audienceAdmin)+string(audienceCustomer)) {
			t.Errorf("feature %s must not carry an audience mark", feature.callback)
		}
	}
}
