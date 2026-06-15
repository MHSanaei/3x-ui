// Package entity defines data structures and entities used by the web layer of the 3x-ui panel.
package entity

import (
	"crypto/tls"
	"math"
	"net"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
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
	WebListen         string `json:"webListen" form:"webListen"`                                     // Web server listen IP address
	WebDomain         string `json:"webDomain" form:"webDomain"`                                     // Web server domain for domain validation
	WebPort           int    `json:"webPort" form:"webPort" validate:"gte=1,lte=65535"`              // Web server port number
	WebCertFile       string `json:"webCertFile" form:"webCertFile"`                                 // Path to SSL certificate file for web server
	WebKeyFile        string `json:"webKeyFile" form:"webKeyFile"`                                   // Path to SSL private key file for web server
	WebBasePath       string `json:"webBasePath" form:"webBasePath"`                                 // Base path for web panel URLs
	SessionMaxAge     int    `json:"sessionMaxAge" form:"sessionMaxAge" validate:"gte=1,lte=525600"` // Session maximum age in minutes (cap at one year)
	TrustedProxyCIDRs string `json:"trustedProxyCIDRs" form:"trustedProxyCIDRs"`                     // Trusted reverse proxy IPs/CIDRs for forwarded headers
	PanelOutbound     string `json:"panelOutbound" form:"panelOutbound"`                             // Xray outbound tag for the panel's own outbound HTTP (update checks/downloads, Telegram, geo updates, outbound-subscription fetches)

	// UI settings
	PageSize    int    `json:"pageSize" form:"pageSize" validate:"gte=0,lte=1000"`      // Number of items per page in lists (0 disables pagination)
	ExpireDiff  int    `json:"expireDiff" form:"expireDiff" validate:"gte=0"`           // Expiration warning threshold in days
	TrafficDiff int    `json:"trafficDiff" form:"trafficDiff" validate:"gte=0,lte=100"` // Traffic warning threshold percentage
	RemarkModel string `json:"remarkModel" form:"remarkModel"`                          // Remark model pattern for inbounds
	Datepicker  string `json:"datepicker" form:"datepicker"`                            // Date picker format

	// Telegram bot settings
	TgBotEnable     bool   `json:"tgBotEnable" form:"tgBotEnable"`              // Enable Telegram bot notifications
	TgBotToken      string `json:"tgBotToken" form:"tgBotToken"`                // Telegram bot token
	TgBotProxy      string `json:"tgBotProxy" form:"tgBotProxy"`                // Proxy URL for Telegram bot
	TgBotAPIServer  string `json:"tgBotAPIServer" form:"tgBotAPIServer"`        // Custom API server for Telegram bot
	TgBotChatId     string `json:"tgBotChatId" form:"tgBotChatId"`              // Telegram chat ID for notifications
	TgRunTime       string `json:"tgRunTime" form:"tgRunTime"`                  // Cron schedule for Telegram notifications
	TgBotBackup     bool   `json:"tgBotBackup" form:"tgBotBackup"`              // Enable database backup via Telegram
	TgCpu           int    `json:"tgCpu" form:"tgCpu" validate:"gte=0,lte=100"` // CPU usage threshold for alerts (percent)
	TgLang          string `json:"tgLang" form:"tgLang"`                        // Telegram bot language
	TgEnabledEvents string `json:"tgEnabledEvents" form:"tgEnabledEvents"`      // Comma-separated event types to send via Telegram

	// Email (SMTP) notification settings
	SmtpEnable         bool   `json:"smtpEnable" form:"smtpEnable"`                        // Enable email notifications
	SmtpHost           string `json:"smtpHost" form:"smtpHost"`                            // SMTP server host
	SmtpPort           int    `json:"smtpPort" form:"smtpPort" validate:"gte=1,lte=65535"` // SMTP server port
	SmtpUsername       string `json:"smtpUsername" form:"smtpUsername"`                    // SMTP username
	SmtpPassword       string `json:"smtpPassword" form:"smtpPassword"`                    // SMTP password
	SmtpTo             string `json:"smtpTo" form:"smtpTo"`                                // Comma-separated recipient emails
	SmtpEncryptionType string `json:"smtpEncryptionType" form:"smtpEncryptionType"`        // SMTP encryption: none, starttls, tls
	SmtpEnabledEvents  string `json:"smtpEnabledEvents" form:"smtpEnabledEvents"`          // Comma-separated event types to send via email
	SmtpCpu            int    `json:"smtpCpu" form:"smtpCpu" validate:"gte=0,lte=100"`     // CPU threshold for email notifications

	// Security settings
	TimeLocation    string `json:"timeLocation" form:"timeLocation"`       // Time zone location
	TwoFactorEnable bool   `json:"twoFactorEnable" form:"twoFactorEnable"` // Enable two-factor authentication
	TwoFactorToken  string `json:"twoFactorToken" form:"twoFactorToken"`   // Two-factor authentication token

	// Subscription server settings
	SubEnable                   bool   `json:"subEnable" form:"subEnable"`                                     // Enable subscription server
	SubJsonEnable               bool   `json:"subJsonEnable" form:"subJsonEnable"`                             // Enable JSON subscription endpoint
	SubTitle                    string `json:"subTitle" form:"subTitle"`                                       // Subscription title
	SubSupportUrl               string `json:"subSupportUrl" form:"subSupportUrl"`                             // Subscription support URL
	SubProfileUrl               string `json:"subProfileUrl" form:"subProfileUrl"`                             // Subscription profile URL
	SubAnnounce                 string `json:"subAnnounce" form:"subAnnounce"`                                 // Subscription announce
	SubEnableRouting            bool   `json:"subEnableRouting" form:"subEnableRouting"`                       // Enable routing for subscription
	SubRoutingRules             string `json:"subRoutingRules" form:"subRoutingRules"`                         // Subscription global routing rules (Only for Happ)
	SubListen                   string `json:"subListen" form:"subListen"`                                     // Subscription server listen IP
	SubPort                     int    `json:"subPort" form:"subPort" validate:"gte=1,lte=65535"`              // Subscription server port
	SubPath                     string `json:"subPath" form:"subPath"`                                         // Base path for subscription URLs
	SubDomain                   string `json:"subDomain" form:"subDomain"`                                     // Domain for subscription server validation
	SubCertFile                 string `json:"subCertFile" form:"subCertFile"`                                 // SSL certificate file for subscription server
	SubKeyFile                  string `json:"subKeyFile" form:"subKeyFile"`                                   // SSL private key file for subscription server
	SubUpdates                  int    `json:"subUpdates" form:"subUpdates" validate:"gte=0,lte=525600"`       // Subscription update interval in minutes
	ExternalTrafficInformEnable bool   `json:"externalTrafficInformEnable" form:"externalTrafficInformEnable"` // Enable external traffic reporting
	ExternalTrafficInformURI    string `json:"externalTrafficInformURI" form:"externalTrafficInformURI"`       // URI for external traffic reporting
	RestartXrayOnClientDisable  bool   `json:"restartXrayOnClientDisable" form:"restartXrayOnClientDisable"`   // Restart Xray when clients are auto-disabled by expiry/traffic limit
	SubEncrypt                  bool   `json:"subEncrypt" form:"subEncrypt"`                                   // Encrypt subscription responses
	SubShowInfo                 bool   `json:"subShowInfo" form:"subShowInfo"`                                 // Show client information in subscriptions
	SubEmailInRemark            bool   `json:"subEmailInRemark" form:"subEmailInRemark"`                       // Include email in subscription remark/name
	SubURI                      string `json:"subURI" form:"subURI"`                                           // Subscription server URI
	SubJsonPath                 string `json:"subJsonPath" form:"subJsonPath"`                                 // Path for JSON subscription endpoint
	SubJsonURI                  string `json:"subJsonURI" form:"subJsonURI"`                                   // JSON subscription server URI
	SubClashEnable              bool   `json:"subClashEnable" form:"subClashEnable"`                           // Enable Clash/Mihomo subscription endpoint
	SubClashPath                string `json:"subClashPath" form:"subClashPath"`                               // Path for Clash/Mihomo subscription endpoint
	SubClashURI                 string `json:"subClashURI" form:"subClashURI"`                                 // Clash/Mihomo subscription server URI
	SubClashEnableRouting       bool   `json:"subClashEnableRouting" form:"subClashEnableRouting"`             // Enable global routing rules for Clash/Mihomo
	SubClashRules               string `json:"subClashRules" form:"subClashRules"`                             // Clash/Mihomo global routing rules
	SubJsonMux                  string `json:"subJsonMux" form:"subJsonMux"`                                   // JSON subscription mux configuration
	SubJsonRules                string `json:"subJsonRules" form:"subJsonRules"`
	SubJsonFinalMask            string `json:"subJsonFinalMask" form:"subJsonFinalMask"` // JSON subscription global finalmask (tcp/udp masks + quicParams)
	SubThemeDir                 string `json:"subThemeDir" form:"subThemeDir"`           // Absolute path to a folder containing a custom subscription page template

	// LDAP settings
	LdapEnable     bool   `json:"ldapEnable" form:"ldapEnable"`
	LdapHost       string `json:"ldapHost" form:"ldapHost"`
	LdapPort       int    `json:"ldapPort" form:"ldapPort" validate:"gte=0,lte=65535"`
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
	LdapDefaultTotalGB    int    `json:"ldapDefaultTotalGB" form:"ldapDefaultTotalGB" validate:"gte=0"`
	LdapDefaultExpiryDays int    `json:"ldapDefaultExpiryDays" form:"ldapDefaultExpiryDays" validate:"gte=0"`
	LdapDefaultLimitIP    int    `json:"ldapDefaultLimitIP" form:"ldapDefaultLimitIP" validate:"gte=0"`
	// JSON subscription routing rules

	// WARP
	WarpUpdateInterval int `json:"warpUpdateInterval" form:"warpUpdateInterval" validate:"gte=0"`
}

// AllSettingView is the browser-safe settings read model. Secret values
// are redacted from the embedded write model and represented by presence
// flags so the UI can show configured/not configured state.
type AllSettingView struct {
	AllSetting

	HasTgBotToken     bool `json:"hasTgBotToken"`
	HasTwoFactorToken bool `json:"hasTwoFactorToken"`
	HasLdapPassword   bool `json:"hasLdapPassword"`
	HasApiToken       bool `json:"hasApiToken"`
	HasWarpSecret     bool `json:"hasWarpSecret"`
	HasNordSecret     bool `json:"hasNordSecret"`
	HasSmtpPassword   bool `json:"hasSmtpPassword"`
}

// CheckValid validates all settings in the AllSetting struct, checking IP addresses, ports, SSL certificates, and other configuration values.
func pathHasForbiddenChar(s string) bool {
	for _, r := range s {
		if r == '\\' || r == ' ' || r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

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

	for _, p := range []struct {
		name  string
		value string
	}{
		{"web base path", s.WebBasePath},
		{"subscription path", s.SubPath},
		{"subscription JSON path", s.SubJsonPath},
		{"subscription Clash path", s.SubClashPath},
	} {
		if pathHasForbiddenChar(p.value) {
			return common.NewError("URI path contains an invalid character:", p.name)
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

	if !strings.HasPrefix(s.SubClashPath, "/") {
		s.SubClashPath = "/" + s.SubClashPath
	}
	if !strings.HasSuffix(s.SubClashPath, "/") {
		s.SubClashPath += "/"
	}

	for cidr := range strings.SplitSeq(s.TrustedProxyCIDRs, ",") {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if ip := net.ParseIP(cidr); ip != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return common.NewError("trusted proxy CIDR is not valid:", cidr)
		}
	}

	_, err := time.LoadLocation(s.TimeLocation)
	if err != nil {
		return common.NewError("time location not exist:", s.TimeLocation)
	}

	return nil
}
