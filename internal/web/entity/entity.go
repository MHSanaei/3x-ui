package entity

import (
	"crypto/tls"
	"math"
	"net"
	"net/mail"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

type Msg struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	Obj     any    `json:"obj"`
}

type AllSetting struct {
	WebListen         string `json:"webListen" form:"webListen"`
	WebDomain         string `json:"webDomain" form:"webDomain"`
	WebPort           int    `json:"webPort" form:"webPort" validate:"gte=1,lte=65535"`
	WebCertFile       string `json:"webCertFile" form:"webCertFile"`
	WebKeyFile        string `json:"webKeyFile" form:"webKeyFile"`
	WebBasePath       string `json:"webBasePath" form:"webBasePath"`
	SessionMaxAge     int    `json:"sessionMaxAge" form:"sessionMaxAge" validate:"gte=1,lte=525600"`
	TrustedProxyCIDRs string `json:"trustedProxyCIDRs" form:"trustedProxyCIDRs"`
	PanelOutbound     string `json:"panelOutbound" form:"panelOutbound"`

	PageSize       int    `json:"pageSize" form:"pageSize" validate:"gte=0,lte=1000"`
	ExpireDiff     int    `json:"expireDiff" form:"expireDiff" validate:"gte=0"`
	TrafficDiff    int    `json:"trafficDiff" form:"trafficDiff" validate:"gte=0,lte=100"`
	RemarkTemplate string `json:"remarkTemplate" form:"remarkTemplate"`
	Datepicker     string `json:"datepicker" form:"datepicker"`

	TgBotEnable     bool   `json:"tgBotEnable" form:"tgBotEnable"`
	TgBotToken      string `json:"tgBotToken" form:"tgBotToken"`
	TgBotProxy      string `json:"tgBotProxy" form:"tgBotProxy"`
	TgBotAPIServer  string `json:"tgBotAPIServer" form:"tgBotAPIServer"`
	TgBotChatId     string `json:"tgBotChatId" form:"tgBotChatId"`
	TgRunTime       string `json:"tgRunTime" form:"tgRunTime"`
	TgBotBackup     bool   `json:"tgBotBackup" form:"tgBotBackup"`
	TgCpu           int    `json:"tgCpu" form:"tgCpu" validate:"gte=0,lte=100"`
	TgMemory        int    `json:"tgMemory" form:"tgMemory" validate:"gte=0,lte=100"`
	TgLang          string `json:"tgLang" form:"tgLang"`
	TgEnabledEvents string `json:"tgEnabledEvents" form:"tgEnabledEvents"`

	SmtpEnable         bool   `json:"smtpEnable" form:"smtpEnable"`
	SmtpHost           string `json:"smtpHost" form:"smtpHost"`
	SmtpPort           int    `json:"smtpPort" form:"smtpPort" validate:"gte=1,lte=65535"`
	SmtpUsername       string `json:"smtpUsername" form:"smtpUsername"`
	SmtpPassword       string `json:"smtpPassword" form:"smtpPassword"`
	SmtpFrom           string `json:"smtpFrom" form:"smtpFrom"`
	SmtpFromName       string `json:"smtpFromName" form:"smtpFromName"`
	SmtpTo             string `json:"smtpTo" form:"smtpTo"`
	SmtpEncryptionType string `json:"smtpEncryptionType" form:"smtpEncryptionType"`
	SmtpEnabledEvents  string `json:"smtpEnabledEvents" form:"smtpEnabledEvents"`
	SmtpCpu            int    `json:"smtpCpu" form:"smtpCpu" validate:"gte=0,lte=100"`
	SmtpMemory         int    `json:"smtpMemory" form:"smtpMemory" validate:"gte=0,lte=100"`

	OutboundDownThreshold int `json:"outboundDownThreshold" form:"outboundDownThreshold" validate:"gte=1,lte=100"`

	TimeLocation    string `json:"timeLocation" form:"timeLocation"`
	TwoFactorEnable bool   `json:"twoFactorEnable" form:"twoFactorEnable"`
	TwoFactorToken  string `json:"twoFactorToken" form:"twoFactorToken"`

	SubEnable                   bool   `json:"subEnable" form:"subEnable"`
	SubJsonEnable               bool   `json:"subJsonEnable" form:"subJsonEnable"`
	SubJsonAutoDetect           bool   `json:"subJsonAutoDetect" form:"subJsonAutoDetect"`
	SubJsonAlwaysArray          bool   `json:"subJsonAlwaysArray" form:"subJsonAlwaysArray"`
	SubJsonUserAgentRegex       string `json:"subJsonUserAgentRegex" form:"subJsonUserAgentRegex"`
	SubClashAutoDetect          bool   `json:"subClashAutoDetect" form:"subClashAutoDetect"`
	SubClashUserAgentRegex      string `json:"subClashUserAgentRegex" form:"subClashUserAgentRegex"`
	SubTitle                    string `json:"subTitle" form:"subTitle"`
	SubSupportUrl               string `json:"subSupportUrl" form:"subSupportUrl"`
	SubProfileUrl               string `json:"subProfileUrl" form:"subProfileUrl"`
	SubAnnounce                 string `json:"subAnnounce" form:"subAnnounce"`
	SubEnableRouting            bool   `json:"subEnableRouting" form:"subEnableRouting"`
	SubRoutingRules             string `json:"subRoutingRules" form:"subRoutingRules"`
	SubIncyEnableRouting        bool   `json:"subIncyEnableRouting" form:"subIncyEnableRouting"`
	SubIncyRoutingRules         string `json:"subIncyRoutingRules" form:"subIncyRoutingRules"`
	SubListen                   string `json:"subListen" form:"subListen"`
	SubPort                     int    `json:"subPort" form:"subPort" validate:"gte=1,lte=65535"`
	SubPath                     string `json:"subPath" form:"subPath"`
	SubDomain                   string `json:"subDomain" form:"subDomain"`
	SubCertFile                 string `json:"subCertFile" form:"subCertFile"`
	SubKeyFile                  string `json:"subKeyFile" form:"subKeyFile"`
	SubUpdates                  int    `json:"subUpdates" form:"subUpdates" validate:"gte=0,lte=525600"`
	ExternalTrafficInformEnable bool   `json:"externalTrafficInformEnable" form:"externalTrafficInformEnable"`
	ExternalTrafficInformURI    string `json:"externalTrafficInformURI" form:"externalTrafficInformURI"`
	RestartXrayOnClientDisable  bool   `json:"restartXrayOnClientDisable" form:"restartXrayOnClientDisable"`
	SubEncrypt                  bool   `json:"subEncrypt" form:"subEncrypt"`
	SubURI                      string `json:"subURI" form:"subURI"`
	SubJsonPath                 string `json:"subJsonPath" form:"subJsonPath"`
	SubJsonURI                  string `json:"subJsonURI" form:"subJsonURI"`
	SubClashEnable              bool   `json:"subClashEnable" form:"subClashEnable"`
	SubClashPath                string `json:"subClashPath" form:"subClashPath"`
	SubClashURI                 string `json:"subClashURI" form:"subClashURI"`
	SubClashEnableRouting       bool   `json:"subClashEnableRouting" form:"subClashEnableRouting"`
	SubClashRules               string `json:"subClashRules" form:"subClashRules"`
	SubJsonMux                  string `json:"subJsonMux" form:"subJsonMux"`
	SubJsonRules                string `json:"subJsonRules" form:"subJsonRules"`
	SubJsonFinalMask            string `json:"subJsonFinalMask" form:"subJsonFinalMask"`
	SubThemeDir                 string `json:"subThemeDir" form:"subThemeDir"`
	SubHideSettings             bool   `json:"subHideSettings" form:"subHideSettings"`

	LdapEnable             bool   `json:"ldapEnable" form:"ldapEnable"`
	LdapHost               string `json:"ldapHost" form:"ldapHost"`
	LdapPort               int    `json:"ldapPort" form:"ldapPort" validate:"gte=0,lte=65535"`
	LdapUseTLS             bool   `json:"ldapUseTLS" form:"ldapUseTLS"`
	LdapInsecureSkipVerify bool   `json:"ldapInsecureSkipVerify" form:"ldapInsecureSkipVerify"`
	LdapBindDN             string `json:"ldapBindDN" form:"ldapBindDN"`
	LdapPassword           string `json:"ldapPassword" form:"ldapPassword"`
	LdapBaseDN             string `json:"ldapBaseDN" form:"ldapBaseDN"`
	LdapUserFilter         string `json:"ldapUserFilter" form:"ldapUserFilter"`
	LdapUserAttr           string `json:"ldapUserAttr" form:"ldapUserAttr"`
	LdapVlessField         string `json:"ldapVlessField" form:"ldapVlessField"`
	LdapSyncCron           string `json:"ldapSyncCron" form:"ldapSyncCron"`
	LdapFlagField          string `json:"ldapFlagField" form:"ldapFlagField"`
	LdapTruthyValues       string `json:"ldapTruthyValues" form:"ldapTruthyValues"`
	LdapInvertFlag         bool   `json:"ldapInvertFlag" form:"ldapInvertFlag"`
	LdapInboundTags        string `json:"ldapInboundTags" form:"ldapInboundTags"`
	LdapAutoCreate         bool   `json:"ldapAutoCreate" form:"ldapAutoCreate"`
	LdapAutoDelete         bool   `json:"ldapAutoDelete" form:"ldapAutoDelete"`
	LdapDefaultTotalGB     int    `json:"ldapDefaultTotalGB" form:"ldapDefaultTotalGB" validate:"gte=0"`
	LdapDefaultExpiryDays  int    `json:"ldapDefaultExpiryDays" form:"ldapDefaultExpiryDays" validate:"gte=0"`
	LdapDefaultLimitIP     int    `json:"ldapDefaultLimitIP" form:"ldapDefaultLimitIP" validate:"gte=0"`

	WarpUpdateInterval int `json:"warpUpdateInterval" form:"warpUpdateInterval" validate:"gte=0"`
}

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

	if (s.SubPort == s.WebPort) && listenAddressesConflict(s.WebListen, s.SubListen) {
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

	if s.SmtpFrom != "" {
		if _, err := mail.ParseAddress(s.SmtpFrom); err != nil {
			return common.NewError("SMTP from address is not valid:", s.SmtpFrom)
		}
	}

	return nil
}

// listenAddressesConflict reports whether two listen addresses on the same port
// would collide at bind time. A wildcard listen ("", "0.0.0.0", "::") overlaps
// every address, so it conflicts with anything on that port; two specific
// addresses conflict only when identical.
func listenAddressesConflict(a, b string) bool {
	if a == b {
		return true
	}
	return isWildcardListen(a) || isWildcardListen(b)
}

func isWildcardListen(listen string) bool {
	if listen == "" {
		return true
	}
	if ip := net.ParseIP(listen); ip != nil {
		return ip.IsUnspecified()
	}
	return false
}

type HostGroup struct {
	GroupId    string   `json:"groupId"`
	InboundIds []int    `json:"inboundIds" validate:"required,min=1"`
	Hosts      []string `json:"hosts" validate:"omitempty"`

	SortOrder              int      `json:"sortOrder"`
	Remark                 string   `json:"remark" validate:"required,max=256"`
	ServerDescription      string   `json:"serverDescription" validate:"omitempty,max=64"`
	IsDisabled             bool     `json:"isDisabled"`
	IsHidden               bool     `json:"isHidden"`
	Tags                   []string `json:"tags"`
	Port                   int      `json:"port" validate:"gte=0,lte=65535"`
	Security               string   `json:"security" validate:"omitempty,oneof=same tls none reality"`
	Sni                    string   `json:"sni"`
	HostHeader             string   `json:"hostHeader"`
	Path                   string   `json:"path"`
	Alpn                   []string `json:"alpn"`
	Fingerprint            string   `json:"fingerprint"`
	OverrideSniFromAddress bool     `json:"overrideSniFromAddress"`
	KeepSniBlank           bool     `json:"keepSniBlank"`
	PinnedPeerCertSha256   []string `json:"pinnedPeerCertSha256"`
	VerifyPeerCertByName   string   `json:"verifyPeerCertByName"`
	AllowInsecure          bool     `json:"allowInsecure"`
	EchConfigList          string   `json:"echConfigList"`
	MuxParams              string   `json:"muxParams"`
	SockoptParams          string   `json:"sockoptParams"`
	FinalMask              string   `json:"finalMask"`
	VlessRoute             string   `json:"vlessRoute"`
	ExcludeFromSubTypes    []string `json:"excludeFromSubTypes"`
	NodeGuids              []string `json:"nodeGuids"`
	MihomoIpVersion        string   `json:"mihomoIpVersion" validate:"omitempty,oneof=dual ipv4 ipv6 ipv4-prefer ipv6-prefer"`
	MihomoX25519           bool     `json:"mihomoX25519"`
	ShuffleHost            bool     `json:"shuffleHost"`
}
