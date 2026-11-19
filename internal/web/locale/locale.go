// Package locale provides internationalization (i18n) support for the 3x-ui web panel,
// including translation loading, localization, and middleware for web and bot interfaces.
package locale

import (
	"encoding/json"
	"io/fs"
	"os"
	"strings"
	"sync/atomic"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// Each is published whole: InitLocalizer runs again (Telegram bot start) while requests read them.
var (
	i18nBundle   atomic.Pointer[i18n.Bundle]
	localizerWeb atomic.Pointer[i18n.Localizer]
	localizerBot atomic.Pointer[i18n.Localizer]
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

// I18n retrieves a localized message for the given key and type.
// It supports both bot and web contexts, with optional template parameters.
// Returns the localized message or an empty string if localization fails.
func I18n(i18nType I18nType, key string, params ...string) string {
	var localizer *i18n.Localizer

	switch i18nType {
	case "bot":
		localizer = localizerBot.Load()
	case "web":
		localizer = localizerWeb.Load()
	default:
		logger.Errorf("Invalid type for I18n: %s", i18nType)
		return ""
	}

	templateData := createTemplateData(params)

	if localizer == nil {
		// Fallback to key if localizer not ready; prevents nil panic on pages like sub
		return key
	}

	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: templateData,
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

	localizerBot.Store(i18n.NewLocalizer(i18nBundle.Load(), botLang))
	return nil
}

// LocalizerMiddleware returns a Gin middleware that sets up localization for web requests.
// It determines the user's language from cookies or Accept-Language header,
// creates a localizer instance, and stores it in the Gin context for use in handlers.
// Also provides the I18n function in the context for template rendering.
func LocalizerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ensure bundle is initialized so creating a Localizer won't panic
		bundle := i18nBundle.Load()
		if bundle == nil {
			bundle = i18n.NewBundle(language.MustParse("en-US"))
			bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
			// Try lazy-load from disk when running sub server without InitLocalizer
			if err := loadTranslationsFromDisk(bundle); err != nil {
				logger.Warning("i18n lazy load failed:", err)
			}
			if !i18nBundle.CompareAndSwap(nil, bundle) {
				bundle = i18nBundle.Load()
			}
		}
		var lang string

		if cookie, err := c.Request.Cookie("lang"); err == nil {
			lang = cookie.Value
		} else {
			lang = c.GetHeader("Accept-Language")
		}

		localizer := i18n.NewLocalizer(bundle, lang)
		localizerWeb.Store(localizer)

		c.Set("localizer", localizer)
		c.Set("I18n", I18n)
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

// SwapBotLocalizer installs l as the bot localizer and returns the previous one.
func SwapBotLocalizer(l *i18n.Localizer) *i18n.Localizer {
	return localizerBot.Swap(l)
}
