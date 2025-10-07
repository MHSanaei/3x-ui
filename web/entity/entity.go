// Package entity defines data structures and entities used by the web layer of the 3x-ui panel.
package entity

import (
	"crypto/tls"
	"math"
	"net"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/util/common"
)

// Msg represents a standard API response message with success status, message text, and optional data object.
type Msg struct {
	Success bool   `json:"success"` // Indicates if the operation was successful
	Msg     string `json:"msg"`     // Response message text
	Obj     any    `json:"obj"`     // Optional data object
}

// AllSetting contains all configuration settings for the 3x-ui panel including web server, Telegram bot, and subscription settings.
type AllSetting struct {
	// Web server settings
	WebListen     string `json:"webListen" form:"webListen"`         // Web server listen IP address
	WebDomain     string `json:"webDomain" form:"webDomain"`         // Web server domain for domain validation
	WebPort       int    `json:"webPort" form:"webPort"`             // Web server port number
	WebCertFile   string `json:"webCertFile" form:"webCertFile"`     // Path to SSL certificate file for web server
	WebKeyFile    string `json:"webKeyFile" form:"webKeyFile"`       // Path to SSL private key file for web server
	WebBasePath   string `json:"webBasePath" form:"webBasePath"`     // Base path for web panel URLs
	SessionMaxAge int    `json:"sessionMaxAge" form:"sessionMaxAge"` // Session maximum age in minutes

	// UI settings
	PageSize    int    `json:"pageSize" form:"pageSize"`       // Number of items per page in lists
	ExpireDiff  int    `json:"expireDiff" form:"expireDiff"`   // Expiration warning threshold in days
	TrafficDiff int    `json:"trafficDiff" form:"trafficDiff"` // Traffic warning threshold percentage
	RemarkModel string `json:"remarkModel" form:"remarkModel"` // Remark model pattern for inbounds
	Datepicker  string `json:"datepicker" form:"datepicker"`   // Date picker format

	// Telegram bot settings
	TgBotEnable      bool   `json:"tgBotEnable" form:"tgBotEnable"`           // Enable Telegram bot notifications
	TgBotToken       string `json:"tgBotToken" form:"tgBotToken"`             // Telegram bot token
	TgBotProxy       string `json:"tgBotProxy" form:"tgBotProxy"`             // Proxy URL for Telegram bot
	TgBotAPIServer   string `json:"tgBotAPIServer" form:"tgBotAPIServer"`     // Custom API server for Telegram bot
	TgBotChatId      string `json:"tgBotChatId" form:"tgBotChatId"`           // Telegram chat ID for notifications
	TgRunTime        string `json:"tgRunTime" form:"tgRunTime"`               // Cron schedule for Telegram notifications
	TgBotBackup      bool   `json:"tgBotBackup" form:"tgBotBackup"`           // Enable database backup via Telegram
	TgBotLoginNotify bool   `json:"tgBotLoginNotify" form:"tgBotLoginNotify"` // Send login notifications
	TgCpu            int    `json:"tgCpu" form:"tgCpu"`                       // CPU usage threshold for alerts
	TgLang           string `json:"tgLang" form:"tgLang"`                     // Telegram bot language

	// Security settings
	TimeLocation    string `json:"timeLocation" form:"timeLocation"`       // Time zone location
	TwoFactorEnable bool   `json:"twoFactorEnable" form:"twoFactorEnable"` // Enable two-factor authentication
	TwoFactorToken  string `json:"twoFactorToken" form:"twoFactorToken"`   // Two-factor authentication token

	// Subscription server settings
	SubEnable                   bool   `json:"subEnable" form:"subEnable"`                                     // Enable subscription server
	SubJsonEnable               bool   `json:"subJsonEnable" form:"subJsonEnable"`                             // Enable JSON subscription endpoint
	SubTitle                    string `json:"subTitle" form:"subTitle"`                                       // Subscription title
	SubListen                   string `json:"subListen" form:"subListen"`                                     // Subscription server listen IP
	SubPort                     int    `json:"subPort" form:"subPort"`                                         // Subscription server port
	SubPath                     string `json:"subPath" form:"subPath"`                                         // Base path for subscription URLs
	SubDomain                   string `json:"subDomain" form:"subDomain"`                                     // Domain for subscription server validation
	SubCertFile                 string `json:"subCertFile" form:"subCertFile"`                                 // SSL certificate file for subscription server
	SubKeyFile                  string `json:"subKeyFile" form:"subKeyFile"`                                   // SSL private key file for subscription server
	SubUpdates                  int    `json:"subUpdates" form:"subUpdates"`                                   // Subscription update interval in minutes
	ExternalTrafficInformEnable bool   `json:"externalTrafficInformEnable" form:"externalTrafficInformEnable"` // Enable external traffic reporting
	ExternalTrafficInformURI    string `json:"externalTrafficInformURI" form:"externalTrafficInformURI"`       // URI for external traffic reporting
	SubEncrypt                  bool   `json:"subEncrypt" form:"subEncrypt"`                                   // Encrypt subscription responses
	SubShowInfo                 bool   `json:"subShowInfo" form:"subShowInfo"`                                 // Show client information in subscriptions
	SubURI                      string `json:"subURI" form:"subURI"`                                           // Subscription server URI
	SubJsonPath                 string `json:"subJsonPath" form:"subJsonPath"`                                 // Path for JSON subscription endpoint
	SubJsonURI                  string `json:"subJsonURI" form:"subJsonURI"`                                   // JSON subscription server URI
	SubJsonFragment             string `json:"subJsonFragment" form:"subJsonFragment"`                         // JSON subscription fragment configuration
	SubJsonNoises               string `json:"subJsonNoises" form:"subJsonNoises"`                             // JSON subscription noise configuration
	SubJsonMux                  string `json:"subJsonMux" form:"subJsonMux"`                                   // JSON subscription mux configuration
	SubJsonRules                string `json:"subJsonRules" form:"subJsonRules"`

	// LDAP settings
	LdapEnable     bool   `json:"ldapEnable" form:"ldapEnable"`
	LdapHost       string `json:"ldapHost" form:"ldapHost"`
	LdapPort       int    `json:"ldapPort" form:"ldapPort"`
	LdapUseTLS     bool   `json:"ldapUseTLS" form:"ldapUseTLS"`
	LdapBindDN     string `json:"ldapBindDN" form:"ldapBindDN"`
	LdapPassword   string `json:"ldapPassword" form:"ldapPassword"`
	LdapBaseDN     string `json:"ldapBaseDN" form:"ldapBaseDN"`
	LdapUserFilter string `json:"ldapUserFilter" form:"ldapUserFilter"`
	LdapUserAttr   string `json:"ldapUserAttr" form:"ldapUserAttr"` // e.g., mail or uid
	LdapVlessField string `json:"ldapVlessField" form:"ldapVlessField"`
	LdapSyncCron   string `json:"ldapSyncCron" form:"ldapSyncCron"`
	// Generic flag configuration
	LdapFlagField         string `json:"ldapFlagField" form:"ldapFlagField"`
	LdapTruthyValues      string `json:"ldapTruthyValues" form:"ldapTruthyValues"`
	LdapInvertFlag        bool   `json:"ldapInvertFlag" form:"ldapInvertFlag"`
	LdapInboundTags       string `json:"ldapInboundTags" form:"ldapInboundTags"`
	LdapAutoCreate        bool   `json:"ldapAutoCreate" form:"ldapAutoCreate"`
	LdapAutoDelete        bool   `json:"ldapAutoDelete" form:"ldapAutoDelete"`
	LdapDefaultTotalGB    int    `json:"ldapDefaultTotalGB" form:"ldapDefaultTotalGB"`
	LdapDefaultExpiryDays int    `json:"ldapDefaultExpiryDays" form:"ldapDefaultExpiryDays"`
	LdapDefaultLimitIP    int    `json:"ldapDefaultLimitIP" form:"ldapDefaultLimitIP"`
	// JSON subscription routing rules
}

// CheckValid validates all settings in the AllSetting struct, checking IP addresses, ports, SSL certificates, and other configuration values.
func (s *AllSetting) CheckValid() error {
	if s.WebListen != "" {
		ip := net.ParseIP(s.WebListen)
		if ip == nil {
			return common.NewError("web listen is not valid ip:", s.WebListen)
		}
	}

	if s.SubListen != "" {
		ip := net.ParseIP(s.SubListen)
		if ip == nil {
			return common.NewError("Sub listen is not valid ip:", s.SubListen)
		}
	}

	if s.WebPort <= 0 || s.WebPort > math.MaxUint16 {
		return common.NewError("web port is not a valid port:", s.WebPort)
	}

	if s.SubPort <= 0 || s.SubPort > math.MaxUint16 {
		return common.NewError("Sub port is not a valid port:", s.SubPort)
	}

	if (s.SubPort == s.WebPort) && (s.WebListen == s.SubListen) {
		return common.NewError("Sub and Web could not use same ip:port, ", s.SubListen, ":", s.SubPort, " & ", s.WebListen, ":", s.WebPort)
	}

	if s.WebCertFile != "" || s.WebKeyFile != "" {
		_, err := tls.LoadX509KeyPair(s.WebCertFile, s.WebKeyFile)
		if err != nil {
			return common.NewErrorf("cert file <%v> or key file <%v> invalid: %v", s.WebCertFile, s.WebKeyFile, err)
		}
	}

	if s.SubCertFile != "" || s.SubKeyFile != "" {
		_, err := tls.LoadX509KeyPair(s.SubCertFile, s.SubKeyFile)
		if err != nil {
			return common.NewErrorf("cert file <%v> or key file <%v> invalid: %v", s.SubCertFile, s.SubKeyFile, err)
		}
	}

	if !strings.HasPrefix(s.WebBasePath, "/") {
		s.WebBasePath = "/" + s.WebBasePath
	}
	if !strings.HasSuffix(s.WebBasePath, "/") {
		s.WebBasePath += "/"
	}
	if !strings.HasPrefix(s.SubPath, "/") {
		s.SubPath = "/" + s.SubPath
	}
	if !strings.HasSuffix(s.SubPath, "/") {
		s.SubPath += "/"
	}

	if !strings.HasPrefix(s.SubJsonPath, "/") {
		s.SubJsonPath = "/" + s.SubJsonPath
	}
	if !strings.HasSuffix(s.SubJsonPath, "/") {
		s.SubJsonPath += "/"
	}

	_, err := time.LoadLocation(s.TimeLocation)
	if err != nil {
		return common.NewError("time location not exist:", s.TimeLocation)
	}

	return nil
}
