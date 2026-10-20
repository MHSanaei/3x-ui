package tgbot

import (
	"slices"
	"testing"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

func TestParseAdminChats(t *testing.T) {
	cases := []struct {
		name   string
		raw    string
		ids    []int64
		topics map[int64]int
	}{
		{name: "empty", raw: "", ids: []int64{}, topics: map[int64]int{}},
		{name: "plain ids", raw: "123,-1001", ids: []int64{123, -1001}, topics: map[int64]int{}},
		{name: "topic", raw: "-1003855200872:42", ids: []int64{-1003855200872}, topics: map[int64]int{-1003855200872: 42}},
		{name: "mixed with spaces", raw: " 123 , -1001 : 7 ", ids: []int64{123, -1001}, topics: map[int64]int{-1001: 7}},
		{name: "duplicate chat collapses", raw: "-1001:7,-1001:7,-1001", ids: []int64{-1001}, topics: map[int64]int{-1001: 7}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ids, topics, err := parseAdminChats(tc.raw)
			if err != nil {
				t.Fatalf("parseAdminChats(%q) error: %v", tc.raw, err)
			}
			if !slices.Equal(ids, tc.ids) {
				t.Fatalf("ids = %v, want %v", ids, tc.ids)
			}
			if len(topics) != len(tc.topics) {
				t.Fatalf("topics = %v, want %v", topics, tc.topics)
			}
			for chat, topic := range tc.topics {
				if topics[chat] != topic {
					t.Fatalf("topics[%d] = %d, want %d", chat, topics[chat], topic)
				}
			}
		})
	}
}

func TestParseAdminChatsRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"abc", "123:", "123:abc", "123:0", "123:-5", "-1001:7,-1001:8", "123,"} {
		if _, _, err := parseAdminChats(raw); err == nil {
			t.Errorf("parseAdminChats(%q) accepted invalid input", raw)
		}
	}
}

func TestInAdminTopicRoutesConfiguredChat(t *testing.T) {
	tgBotMutex.Lock()
	orig := adminTopics
	adminTopics = map[int64]int{-1001: 42}
	tgBotMutex.Unlock()
	t.Cleanup(func() {
		tgBotMutex.Lock()
		adminTopics = orig
		tgBotMutex.Unlock()
	})

	doc := inAdminTopic(tu.Document(tu.ID(-1001), tu.FileFromBytes([]byte("x"), "x.db")))
	if doc.MessageThreadID != 42 {
		t.Fatalf("MessageThreadID = %d, want 42", doc.MessageThreadID)
	}
	doc = inAdminTopic(&telego.SendDocumentParams{ChatID: tu.ID(123)})
	if doc.MessageThreadID != 0 {
		t.Fatalf("MessageThreadID = %d for chat without topic, want 0", doc.MessageThreadID)
	}
}
