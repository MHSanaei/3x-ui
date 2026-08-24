package tgbot

import (
	"strings"
	"testing"
)

func TestPageMessageSplitsLinkListWithoutBlankLines(t *testing.T) {
	var message strings.Builder
	message.WriteString("Individual links:\r\n")
	for range 50 {
		message.WriteString("<code>vless://" + strings.Repeat("a", 300) + "</code>\r\n")
	}

	pages := pageMessage(message.String(), telegramPageLimit)
	if len(pages) < 2 {
		t.Fatalf("pageMessage() returned %d page, want multiple", len(pages))
	}

	links := 0
	for index, page := range pages {
		if len(page) > telegramPageLimit {
			t.Errorf("page %d has %d bytes, want at most %d", index, len(page), telegramPageLimit)
		}
		openingTags := strings.Count(page, "<code>")
		closingTags := strings.Count(page, "</code>")
		if openingTags != closingTags {
			t.Errorf("page %d has %d opening tags and %d closing tags", index, openingTags, closingTags)
		}
		links += openingTags
	}
	if links != 50 {
		t.Errorf("pages contain %d links, want 50", links)
	}
}
