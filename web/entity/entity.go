package entity

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"strings"
	"time"
	"x-ui/util/common"
	"x-ui/xray"
)

type Msg struct {
	Success bool        `json:"success"`
	Msg     string      `json:"msg"`
	Obj     interface{} `json:"obj"`
}

type Pager struct {
	Current  int         `json:"current"`
	PageSize int         `json:"page_size"`
	Total    int         `json:"total"`
	OrderBy  string      `json:"order_by"`
	Desc     bool        `json:"desc"`
	Key      string      `json:"key"`
	List     interface{} `json:"list"`
}

type AllSetting struct {
	WebListen          string `json:"webListen" form:"webListen"`
	WebDomain          string `json:"webDomain" form:"webDomain"`
	WebPort            int    `json:"webPort" form:"webPort"`
	WebCertFile        string `json:"webCertFile" form:"webCertFile"`
	WebKeyFile         string `json:"webKeyFile" form:"webKeyFile"`
	WebBasePath        string `json:"webBasePath" form:"webBasePath"`
	SessionMaxAge      int    `json:"sessionMaxAge" form:"sessionMaxAge"`
	ExpireDiff         int    `json:"expireDiff" form:"expireDiff"`
	TrafficDiff        int    `json:"trafficDiff" form:"trafficDiff"`
	TgBotEnable        bool   `json:"tgBotEnable" form:"tgBotEnable"`
	TgBotToken         string `json:"tgBotToken" form:"tgBotToken"`
	TgBotChatId        string `json:"tgBotChatId" form:"tgBotChatId"`
	TgRunTime          string `json:"tgRunTime" form:"tgRunTime"`
	TgBotBackup        bool   `json:"tgBotBackup" form:"tgBotBackup"`
	TgBotLoginNotify   bool   `json:"tgBotLoginNotify" form:"tgBotLoginNotify"`
	TgCpu              int    `json:"tgCpu" form:"tgCpu"`
	TgLang             string `json:"tgLang" form:"tgLang"`
	XrayTemplateConfig string `json:"xrayTemplateConfig" form:"xrayTemplateConfig"`
	TimeLocation       string `json:"timeLocation" form:"timeLocation"`
	SecretEnable       bool   `json:"secretEnable" form:"secretEnable"`
	SubEnable          bool   `json:"subEnable" form:"subEnable"`
	SubListen          string `json:"subListen" form:"subListen"`
	SubPort            int    `json:"subPort" form:"subPort"`
	SubPath            string `json:"subPath" form:"subPath"`
	SubDomain          string `json:"subDomain" form:"subDomain"`
	SubCertFile        string `json:"subCertFile" form:"subCertFile"`
	SubKeyFile         string `json:"subKeyFile" form:"subKeyFile"`
	SubUpdates         int    `json:"subUpdates" form:"subUpdates"`
	SubEncrypt         bool   `json:"subEncrypt" form:"subEncrypt"`
	SubShowInfo        bool   `json:"subShowInfo" form:"subShowInfo"`
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

	if s.WebPort <= 0 || s.WebPort > 65535 {
		return common.NewError("web port is not a valid port:", s.WebPort)
	}

	if s.SubPort <= 0 || s.SubPort > 65535 {
		return common.NewError("Sub port is not a valid port:", s.SubPort)
	}

	if s.SubPort == s.WebPort {
		return common.NewError("Sub and Web could not use same port:", s.SubPort)
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

	xrayConfig := &xray.Config{}
	err := json.Unmarshal([]byte(s.XrayTemplateConfig), xrayConfig)
	if err != nil {
		return common.NewError("xray template config invalid:", err)
	}

	_, err = time.LoadLocation(s.TimeLocation)
	if err != nil {
		return common.NewError("time location not exist:", s.TimeLocation)
	}

	return nil
}
