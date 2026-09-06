package tgbot

import (
	"encoding/json"
	"strconv"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

// botLanguage is one entry in the picker. The tags mirror the locale files in
// internal/web/translation, and the labels mirror the panel's own language menu.
type botLanguage struct {
	tag   string
	label string
}

var botLanguages = []botLanguage{
	{tag: "en-US", label: "🇺🇸 English"},
	{tag: "fa-IR", label: "🇮🇷 فارسی"},
	{tag: "ar-EG", label: "🇪🇬 العربية"},
	{tag: "ru-RU", label: "🇷🇺 Русский"},
	{tag: "uk-UA", label: "🇺🇦 Український"},
	{tag: "tr-TR", label: "🇹🇷 Türkçe"},
	{tag: "es-ES", label: "🇪🇸 Español"},
	{tag: "pt-BR", label: "🇧🇷 Português"},
	{tag: "id-ID", label: "🇮🇩 Indonesian"},
	{tag: "vi-VN", label: "🇻🇳 Tiếng Việt"},
	{tag: "ja-JP", label: "🇯🇵 日本語"},
	{tag: "zh-CN", label: "🇨🇳 简体中文"},
	{tag: "zh-TW", label: "🇹🇼 繁體中文"},
}

func isSupportedLang(tag string) bool {
	for _, lang := range botLanguages {
		if lang.tag == tag {
			return true
		}
	}
	return false
}

func languageLabel(tag string) string {
	for _, lang := range botLanguages {
		if lang.tag == tag {
			return lang.label
		}
	}
	return tag
}

// The blob is a single settings row, so a read-modify-write from two updates at
// once would lose one of them.
var userLangMu sync.Mutex

// Anything unreadable degrades to "nobody picked a language" rather than failing
// the update: the panel language is always a usable answer.
func parseUserLangs(blob string) map[int64]string {
	langs := map[int64]string{}
	if blob == "" {
		return langs
	}

	raw := map[string]string{}
	if err := json.Unmarshal([]byte(blob), &raw); err != nil {
		logger.Warning("tgbot: user language store is unreadable:", err)
		return langs
	}

	for key, tag := range raw {
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil || !isSupportedLang(tag) {
			continue
		}
		langs[id] = tag
	}
	return langs
}

func encodeUserLangs(langs map[int64]string) (string, error) {
	raw := make(map[string]string, len(langs))
	for id, tag := range langs {
		raw[strconv.FormatInt(id, 10)] = tag
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (t *Tgbot) userLangs() map[int64]string {
	blob, err := t.settingService.GetTgBotUserLangs()
	if err != nil {
		logger.Warning("tgbot: user language lookup failed:", err)
		return map[int64]string{}
	}
	return parseUserLangs(blob)
}

// langOf returns "" for a user who never chose, which every localizer call reads
// as "use the panel language".
func (t *Tgbot) langOf(tgUserID int64) string {
	if tgUserID == 0 {
		return ""
	}
	return t.userLangs()[tgUserID]
}

func (t *Tgbot) setUserLang(tgUserID int64, tag string) error {
	if tgUserID == 0 {
		return common.NewError("no telegram user to set a language for")
	}
	if !isSupportedLang(tag) {
		return common.NewError("unsupported language:", tag)
	}

	userLangMu.Lock()
	defer userLangMu.Unlock()

	langs := t.userLangs()
	langs[tgUserID] = tag
	encoded, err := encodeUserLangs(langs)
	if err != nil {
		return err
	}
	return t.settingService.SetTgBotUserLangs(encoded)
}

// forUser returns a copy of the bot that speaks one user's language, so the 601
// existing I18nBot call sites reach it without changing their signatures.
func (t *Tgbot) forUser(tgUserID int64) *Tgbot {
	scoped := *t
	scoped.lang = t.langOf(tgUserID)
	return &scoped
}
