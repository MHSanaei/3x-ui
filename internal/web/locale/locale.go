// Package locale provides internationalization (i18n) support for the 3x-ui web panel,
// including translation loading, localization, and middleware for web and bot interfaces.
package locale

import (
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	i18nBundle     atomic.Pointer[i18n.Bundle]
	lazyBundleLock sync.Mutex
	LocalizerBot   *i18n.Localizer
)

// I18nType represents the type of interface for internationalization.
type I18nType string

const (
	Bot I18nType = "bot" // Bot interface type
	Web I18nType = "web" // Web interface type
)

// SettingService interface defines methods for accessing locale settings.
type SettingService interface {
	GetTgLang() (string, error)
}

// InitLocalizer initializes the internationalization system with embedded translation files.
func InitLocalizer(i18nFS fs.FS, settingService SettingService) error {
	// set default bundle to English
	bundle := i18n.NewBundle(language.MustParse("en-US"))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// parse files
	if err := parseTranslationFiles(i18nFS, bundle); err != nil {
		return err
	}
	i18nBundle.Store(bundle)

	// setup bot locale
	if err := initTGBotLocalizer(settingService); err != nil {
		return err
	}

	return nil
}

// createTemplateData creates a template data map from parameters with optional separator.
func createTemplateData(params []string, separator ...string) map[string]any {
	sep := "=="
	if len(separator) > 0 {
		sep = separator[0]
	}

	templateData := make(map[string]any)
	for _, param := range params {
		parts := strings.SplitN(param, sep, 2)
		templateData[parts[0]] = parts[1]
	}

	return templateData
}

// I18n localizes a bot message. Web messages go through the per-request function
// LocalizerMiddleware stores in the Gin context; here Web has no language and yields the key.
func I18n(i18nType I18nType, key string, params ...string) string {
	switch i18nType {
	case Bot:
		return localize(LocalizerBot, key, params)
	case Web:
		return key
	default:
		logger.Errorf("Invalid type for I18n: %s", i18nType)
		return ""
	}
}

func localize(localizer *i18n.Localizer, key string, params []string) string {
	if localizer == nil {
		// Fallback to key if localizer not ready; prevents nil panic on pages like sub
		return key
	}

	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: createTemplateData(params),
	})
	if err != nil {
		logger.Errorf("Failed to localize message: %v", err)
		return ""
	}

	return msg
}

// LocalizerFor returns a new Localizer for the given language tag using the global bundle.
func LocalizerFor(lang string) *i18n.Localizer {
	bundle := i18nBundle.Load()
	if bundle == nil {
		return nil
	}
	return i18n.NewLocalizer(bundle, lang)
}

// I18nForLang retrieves a localized message for a specific language tag with optional params.
func I18nForLang(lang string, key string, params ...string) string {
	loc := LocalizerFor(lang)
	if loc == nil {
		return key
	}
	templateData := createTemplateData(params)
	msg, err := loc.Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: templateData,
	})
	if err != nil {
		return key
	}
	return msg
}

// initTGBotLocalizer initializes the bot localizer with the configured language.
func initTGBotLocalizer(settingService SettingService) error {
	botLang, err := settingService.GetTgLang()
	if err != nil {
		return err
	}

	LocalizerBot = LocalizerFor(botLang)
	return nil
}

// requestBundle returns the bundle, lazily loading it from disk for a sub
// server that runs without InitLocalizer.
func requestBundle() *i18n.Bundle {
	if bundle := i18nBundle.Load(); bundle != nil {
		return bundle
	}
	lazyBundleLock.Lock()
	defer lazyBundleLock.Unlock()
	if bundle := i18nBundle.Load(); bundle != nil {
		return bundle
	}
	bundle := i18n.NewBundle(language.MustParse("en-US"))
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	if err := loadTranslationsFromDisk(bundle); err != nil {
		logger.Warning("i18n lazy load failed:", err)
	}
	i18nBundle.Store(bundle)
	return bundle
}

// LocalizerMiddleware picks the request language from the lang cookie or Accept-Language
// and stores in the Gin context a localizer and an I18n function bound to that language.
func LocalizerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var lang string

		if cookie, err := c.Request.Cookie("lang"); err == nil {
			lang = cookie.Value
		} else {
			lang = c.GetHeader("Accept-Language")
		}

		localizer := i18n.NewLocalizer(requestBundle(), lang)

		c.Set("localizer", localizer)
		c.Set("I18n", func(i18nType I18nType, key string, params ...string) string {
			if i18nType == Web {
				return localize(localizer, key, params)
			}
			return I18n(i18nType, key, params...)
		})
		c.Next()
	}
}

// loadTranslationsFromDisk attempts to load translation files from "internal/web/translation" using the local filesystem.
func loadTranslationsFromDisk(bundle *i18n.Bundle) error {
	root := os.DirFS("internal/web")
	return fs.WalkDir(root, "translation", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		_, err = bundle.ParseMessageFileBytes(data, path)
		return err
	})
}

// parseTranslationFiles parses embedded translation files and adds them to the i18n bundle.
func parseTranslationFiles(i18nFS fs.FS, i18nBundle *i18n.Bundle) error {
	err := fs.WalkDir(i18nFS, "translation",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			data, err := fs.ReadFile(i18nFS, path)
			if err != nil {
				return err
			}

			_, err = i18nBundle.ParseMessageFileBytes(data, path)
			return err
		})
	if err != nil {
		return err
	}

	return nil
}
