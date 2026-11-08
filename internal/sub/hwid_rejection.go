package sub

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type hwidResponseFormat int

const (
	hwidLinks hwidResponseFormat = iota
	hwidJSON
	hwidJSONArray
	hwidClash
)

// A parseable dummy node lets subscription apps replace previously downloaded nodes.
// Never reuse subscriber credentials or include a direct-connection fallback here.
func (a *SUBController) writeHwidRejection(c *gin.Context, result service.HwidGateResult, format hwidResponseFormat) {
	enabled, err := a.settingService.GetSubHwidRejectWithConfig()
	if err != nil {
		writeSubError(c, err)
		return
	}
	if !enabled {
		applyHwidHeaders(c, result)
		c.Status(http.StatusNotFound)
		return
	}
	remark := "Device limit reached"
	if result.NotSupported {
		remark = "HWID required"
	}
	contentType := "text/plain; charset=utf-8"
	var body string
	switch format {
	case hwidJSON, hwidJSONArray:
		contentType = "application/json; charset=utf-8"
		body = fmt.Sprintf(`{"remarks":%q,"inbounds":[{"listen":"127.0.0.1","port":10808,"protocol":"socks","settings":{"auth":"noauth","udp":true}}],"outbounds":[{"tag":"blocked","protocol":"blackhole","settings":{}}]}`, remark)
		if format == hwidJSONArray || a.jsonAlwaysArray {
			body = "[" + body + "]"
		}
	case hwidClash:
		contentType = "application/yaml; charset=utf-8"
		body = fmt.Sprintf("mode: rule\nproxies:\n  - name: %q\n    type: vmess\n    server: 192.0.2.1\n    port: 1\n    uuid: 00000000-0000-4000-8000-000000000001\n    alterId: 0\n    cipher: auto\nproxy-groups:\n  - name: HWID-BLOCKED\n    type: select\n    proxies:\n      - %q\nrules:\n  - MATCH,HWID-BLOCKED\n", remark, remark)
	default:
		body = "vless://00000000-0000-4000-8000-000000000001@192.0.2.1:1?encryption=none&type=tcp&security=none#" + url.PathEscape(remark) + "\n"
		if a.subEncrypt {
			body = base64.StdEncoding.EncodeToString([]byte(body))
		}
	}
	c.Header("Profile-Update-Interval", a.updateInterval)
	// Rejection headers can make apps discard a successful replacement response.
	applyHwidHeaders(c, service.HwidGateResult{Active: result.Active})
	c.Header("Announce", "base64:"+base64.StdEncoding.EncodeToString([]byte(remark)))
	c.Header("Content-Type", contentType)
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}
	c.Data(http.StatusOK, contentType, []byte(body))
}
