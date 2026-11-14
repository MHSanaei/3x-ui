package sub

import (
	"encoding/base64"
	"net/http"

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
	node := statusNode{remark: remark, blocked: true}
	contentType := "text/plain; charset=utf-8"
	var body string
	switch format {
	case hwidJSON, hwidJSONArray:
		contentType = "application/json; charset=utf-8"
		body = string(a.subJsonService.genStatusNodeConfig(node))
		if format == hwidJSONArray || a.jsonAlwaysArray {
			body = "[" + body + "]"
		}
	case hwidClash:
		contentType = "application/yaml; charset=utf-8"
		config := newClashConfig([]map[string]any{node.clashProxy()}, false)
		yaml, err := marshalClashYAML(config)
		if err != nil {
			writeSubError(c, err)
			return
		}
		body = string(yaml)
	default:
		body = node.link() + "\n"
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
