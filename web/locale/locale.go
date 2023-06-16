package locale

import (
	"embed"
	"io/fs"
	"strings"
	"x-ui/logger"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/language"
)

var i18nBundle *i18n.Bundle
var LocalizerWeb *i18n.Localizer
var LocalizerBot *i18n.Localizer

type I18nType string

const (
	Bot I18nType = "bot"
	Web I18nType = "web"
)

type SettingService interface {
	GetTgLang() (string, error)
}

func InitLocalizer(i18nFS embed.FS, settingService SettingService) error {
	// set default bundle to english
	i18nBundle = i18n.NewBundle(language.MustParse("en-US"))
	i18nBundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	// parse files
	if err := parseTranslationFiles(i18nFS, i18nBundle); err != nil {
		return err
	}

	// setup bot locale
	if err := initTGBotLocalizer(settingService); err != nil {
		return err
	}

	return nil
}

func createTemplateData(params []string, seperator ...string) map[string]interface{} {
	var sep string = "=="
	if len(seperator) > 0 {
		sep = seperator[0]
	}

	templateData := make(map[string]interface{})
	for _, param := range params {
		parts := strings.SplitN(param, sep, 2)
		templateData[parts[0]] = parts[1]
	}

	return templateData
}

func I18n(i18nType I18nType, key string, params ...string) string {
	var localizer *i18n.Localizer

	switch i18nType {
	case "bot":
		localizer = LocalizerBot
	case "web":
		localizer = LocalizerWeb
	default:
		logger.Errorf("Invalid type for I18n: %s", i18nType)
		return ""
	}

	templateData := createTemplateData(params)

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

func initTGBotLocalizer(settingService SettingService) error {
	botLang, err := settingService.GetTgLang()
	if err != nil {
		return err
	}

	LocalizerBot = i18n.NewLocalizer(i18nBundle, botLang)
	return nil
}

func LocalizerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var lang string

		if cookie, err := c.Request.Cookie("lang"); err == nil {
			lang = cookie.Value
		} else {
			lang = c.GetHeader("Accept-Language")
		}

		LocalizerWeb = i18n.NewLocalizer(i18nBundle, lang)

		c.Set("localizer", LocalizerWeb)
		c.Set("I18n", I18n)
		c.Next()
	}
}

func parseTranslationFiles(i18nFS embed.FS, i18nBundle *i18n.Bundle) error {
	err := fs.WalkDir(i18nFS, "translation",
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			data, err := i18nFS.ReadFile(path)
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
