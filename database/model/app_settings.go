package model

import (
	"fmt"
	"reflect"
	"strconv"
	"sync"

	"github.com/mhsanaei/3x-ui/v2/util/random"
)

// AppSettings is the authoritative typed settings row.
// It mirrors the historical key/value settings keys via `setting` tags.
type AppSettings struct {
	ID        int   `gorm:"primaryKey;autoIncrement"`
	CreatedAt int64 `gorm:"autoCreateTime:milli"`
	UpdatedAt int64 `gorm:"autoUpdateTime:milli"`

	XrayTemplateConfig string `gorm:"type:text" setting:"xrayTemplateConfig"`
	WebListen          string `setting:"webListen"`
	WebDomain          string `setting:"webDomain"`
	WebPort            int    `setting:"webPort"`
	WebCertFile        string `setting:"webCertFile"`
	WebKeyFile         string `setting:"webKeyFile"`
	Secret             string `setting:"secret"`
	WebBasePath        string `setting:"webBasePath"`
	SessionMaxAge      int    `setting:"sessionMaxAge"`
	PageSize           int    `setting:"pageSize"`
	ExpireDiff         int    `setting:"expireDiff"`
	TrafficDiff        int    `setting:"trafficDiff"`
	RemarkModel        string `setting:"remarkModel"`
	TimeLocation       string `setting:"timeLocation"`

	TgBotEnable      bool   `setting:"tgBotEnable"`
	TgBotToken       string `setting:"tgBotToken"`
	TgBotProxy       string `setting:"tgBotProxy"`
	TgBotAPIServer   string `setting:"tgBotAPIServer"`
	TgBotChatID      string `setting:"tgBotChatId"`
	TgRunTime        string `setting:"tgRunTime"`
	TgBotBackup      bool   `setting:"tgBotBackup"`
	TgBotLoginNotify bool   `setting:"tgBotLoginNotify"`
	TgCPU            int    `setting:"tgCpu"`
	TgLang           string `setting:"tgLang"`

	TwoFactorEnable bool   `setting:"twoFactorEnable"`
	TwoFactorToken  string `setting:"twoFactorToken"`

	SubEnable        bool   `setting:"subEnable"`
	SubJSONEnable    bool   `setting:"subJsonEnable"`
	SubTitle         string `setting:"subTitle"`
	SubSupportURL    string `setting:"subSupportUrl"`
	SubProfileURL    string `setting:"subProfileUrl"`
	SubAnnounce      string `setting:"subAnnounce"`
	SubEnableRouting bool   `setting:"subEnableRouting"`
	SubRoutingRules  string `gorm:"type:text" setting:"subRoutingRules"`
	SubListen        string `setting:"subListen"`
	SubPort          int    `setting:"subPort"`
	SubPath          string `setting:"subPath"`
	SubDomain        string `setting:"subDomain"`
	SubCertFile      string `setting:"subCertFile"`
	SubKeyFile       string `setting:"subKeyFile"`
	SubUpdates       string `setting:"subUpdates"`
	SubEncrypt       bool   `setting:"subEncrypt"`
	SubShowInfo      bool   `setting:"subShowInfo"`
	SubURI           string `setting:"subURI"`
	SubJSONPath      string `setting:"subJsonPath"`
	SubJSONURI       string `setting:"subJsonURI"`
	SubJSONFragment  string `setting:"subJsonFragment"`
	SubJSONNoises    string `setting:"subJsonNoises"`
	SubJSONMux       string `setting:"subJsonMux"`
	SubJSONRules     string `setting:"subJsonRules"`
	Datepicker       string `setting:"datepicker"`

	Warp                        string `setting:"warp"`
	ExternalTrafficInformEnable bool   `setting:"externalTrafficInformEnable"`
	ExternalTrafficInformURI    string `setting:"externalTrafficInformURI"`
	XrayOutboundTestURL         string `setting:"xrayOutboundTestUrl"`

	LdapEnable            bool   `setting:"ldapEnable"`
	LdapHost              string `setting:"ldapHost"`
	LdapPort              int    `setting:"ldapPort"`
	LdapUseTLS            bool   `setting:"ldapUseTLS"`
	LdapBindDN            string `setting:"ldapBindDN"`
	LdapPassword          string `setting:"ldapPassword"`
	LdapBaseDN            string `setting:"ldapBaseDN"`
	LdapUserFilter        string `setting:"ldapUserFilter"`
	LdapUserAttr          string `setting:"ldapUserAttr"`
	LdapVlessField        string `setting:"ldapVlessField"`
	LdapSyncCron          string `setting:"ldapSyncCron"`
	LdapFlagField         string `setting:"ldapFlagField"`
	LdapTruthyValues      string `setting:"ldapTruthyValues"`
	LdapInvertFlag        bool   `setting:"ldapInvertFlag"`
	LdapInboundTags       string `setting:"ldapInboundTags"`
	LdapAutoCreate        bool   `setting:"ldapAutoCreate"`
	LdapAutoDelete        bool   `setting:"ldapAutoDelete"`
	LdapDefaultTotalGB    int    `setting:"ldapDefaultTotalGB"`
	LdapDefaultExpiryDays int    `setting:"ldapDefaultExpiryDays"`
	LdapDefaultLimitIP    int    `setting:"ldapDefaultLimitIP"`
}

func (AppSettings) TableName() string {
	return "app_settings"
}

var (
	appSettingsFieldMapOnce sync.Once
	appSettingsFieldMap     map[string]int
)

func buildAppSettingsFieldMap() {
	appSettingsFieldMap = make(map[string]int)
	t := reflect.TypeOf(AppSettings{})
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		key := f.Tag.Get("setting")
		if key == "" {
			continue
		}
		appSettingsFieldMap[key] = i
	}
}

func getAppSettingsFieldMap() map[string]int {
	appSettingsFieldMapOnce.Do(buildAppSettingsFieldMap)
	return appSettingsFieldMap
}

// DefaultSettingValues returns canonical defaults for settings keys.
func DefaultSettingValues(xrayTemplateConfig string) map[string]string {
	return map[string]string{
		"xrayTemplateConfig":          xrayTemplateConfig,
		"webListen":                   "",
		"webDomain":                   "",
		"webPort":                     "2053",
		"webCertFile":                 "",
		"webKeyFile":                  "",
		"secret":                      random.Seq(32),
		"webBasePath":                 "/",
		"sessionMaxAge":               "360",
		"pageSize":                    "25",
		"expireDiff":                  "0",
		"trafficDiff":                 "0",
		"remarkModel":                 "-ieo",
		"timeLocation":                "Local",
		"tgBotEnable":                 "false",
		"tgBotToken":                  "",
		"tgBotProxy":                  "",
		"tgBotAPIServer":              "",
		"tgBotChatId":                 "",
		"tgRunTime":                   "@daily",
		"tgBotBackup":                 "false",
		"tgBotLoginNotify":            "true",
		"tgCpu":                       "80",
		"tgLang":                      "en-US",
		"twoFactorEnable":             "false",
		"twoFactorToken":              "",
		"subEnable":                   "true",
		"subJsonEnable":               "false",
		"subTitle":                    "",
		"subSupportUrl":               "",
		"subProfileUrl":               "",
		"subAnnounce":                 "",
		"subEnableRouting":            "true",
		"subRoutingRules":             "",
		"subListen":                   "",
		"subPort":                     "2096",
		"subPath":                     "/sub/",
		"subDomain":                   "",
		"subCertFile":                 "",
		"subKeyFile":                  "",
		"subUpdates":                  "12",
		"subEncrypt":                  "true",
		"subShowInfo":                 "true",
		"subURI":                      "",
		"subJsonPath":                 "/json/",
		"subJsonURI":                  "",
		"subJsonFragment":             "",
		"subJsonNoises":               "",
		"subJsonMux":                  "",
		"subJsonRules":                "",
		"datepicker":                  "gregorian",
		"warp":                        "",
		"externalTrafficInformEnable": "false",
		"externalTrafficInformURI":    "",
		"xrayOutboundTestUrl":         "https://www.google.com/generate_204",
		"ldapEnable":                  "false",
		"ldapHost":                    "",
		"ldapPort":                    "389",
		"ldapUseTLS":                  "false",
		"ldapBindDN":                  "",
		"ldapPassword":                "",
		"ldapBaseDN":                  "",
		"ldapUserFilter":              "(objectClass=person)",
		"ldapUserAttr":                "mail",
		"ldapVlessField":              "vless_enabled",
		"ldapSyncCron":                "@every 1m",
		"ldapFlagField":               "",
		"ldapTruthyValues":            "true,1,yes,on",
		"ldapInvertFlag":              "false",
		"ldapInboundTags":             "",
		"ldapAutoCreate":              "false",
		"ldapAutoDelete":              "false",
		"ldapDefaultTotalGB":          "0",
		"ldapDefaultExpiryDays":       "0",
		"ldapDefaultLimitIP":          "0",
	}
}

// NewDefaultAppSettings creates a settings row initialized with default values.
func NewDefaultAppSettings(xrayTemplateConfig string) *AppSettings {
	cfg := &AppSettings{}
	defaults := DefaultSettingValues(xrayTemplateConfig)
	for key, value := range defaults {
		_, _ = cfg.SetByKey(key, value)
	}
	return cfg
}

// GetByKey returns a string representation of a settings key value.
func (s *AppSettings) GetByKey(key string) (string, bool, error) {
	idx, ok := getAppSettingsFieldMap()[key]
	if !ok {
		return "", false, nil
	}
	v := reflect.ValueOf(s).Elem().Field(idx)
	switch v.Kind() {
	case reflect.String:
		return v.String(), true, nil
	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), true, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), true, nil
	default:
		return "", true, fmt.Errorf("unsupported settings field kind for key %s: %s", key, v.Kind())
	}
}

// SetByKey sets a settings value by historical key name.
func (s *AppSettings) SetByKey(key string, value string) (bool, error) {
	idx, ok := getAppSettingsFieldMap()[key]
	if !ok {
		return false, nil
	}
	v := reflect.ValueOf(s).Elem().Field(idx)
	switch v.Kind() {
	case reflect.String:
		v.SetString(value)
		return true, nil
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return true, err
		}
		v.SetBool(parsed)
		return true, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return true, err
		}
		v.SetInt(parsed)
		return true, nil
	default:
		return true, fmt.Errorf("unsupported settings field kind for key %s: %s", key, v.Kind())
	}
}
