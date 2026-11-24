package controller

import (
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/clientip"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

// getRemoteIp is the request's client address, the identity the login limiter keys on.
func getRemoteIp(c *gin.Context) string {
	return clientip.Resolve(c.Request.RemoteAddr, c.GetHeader("X-Real-IP"), c.GetHeader("X-Forwarded-For"), trustedProxyCIDRs())
}

func isTrustedForwardedRequest(c *gin.Context) bool {
	remoteIP, ok := clientip.Parse(c.Request.RemoteAddr)
	return ok && clientip.Trusted(remoteIP, trustedProxyCIDRs())
}

func trustedProxyCIDRs() (trusted string) {
	trusted = service.DefaultTrustedProxyCIDRs
	defer func() {
		_ = recover()
	}()
	settingService := service.SettingService{}
	if value, err := settingService.GetTrustedProxyCIDRs(); err == nil && strings.TrimSpace(value) != "" {
		trusted = value
	}
	return trusted
}

// jsonMsg sends a JSON response with a message and error status.
func jsonMsg(c *gin.Context, msg string, err error) {
	jsonMsgObj(c, msg, nil, err)
}

// jsonObj sends a JSON response with an object and error status.
func jsonObj(c *gin.Context, obj any, err error) {
	jsonMsgObj(c, "", obj, err)
}

func requestErrorContext(c *gin.Context) string {
	handler, loc := callerOutsideUtil()
	return fmt.Sprintf("[%s %s handler=%s %s]", c.Request.Method, c.Request.URL.Path, handler, loc)
}

func callerOutsideUtil() (string, string) {
	var pcs [12]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		base := filepath.Base(frame.File)
		if base != "util.go" {
			name := frame.Function
			if idx := strings.LastIndex(name, "/"); idx >= 0 {
				name = name[idx+1:]
			}
			return name, fmt.Sprintf("%s:%d", base, frame.Line)
		}
		if !more {
			break
		}
	}
	return "unknown", "unknown"
}

// jsonMsgObj sends a JSON response with a message, object, and error status.
func jsonMsgObj(c *gin.Context, msg string, obj any, err error) {
	m := entity.Msg{
		Obj: obj,
	}
	if err == nil {
		m.Success = true
		if msg != "" {
			m.Msg = msg
		}
	} else {
		m.Success = false
		ctx := requestErrorContext(c)
		fail := I18nWeb(c, "fail")
		errStr := err.Error()
		if errStr != "" {
			m.Msg = msg + " (" + errStr + ")"
			logger.Warningf("%s %s %s: %v", ctx, msg, fail, err)
		} else if msg != "" {
			m.Msg = msg
			logger.Warningf("%s %s %s", ctx, msg, fail)
		} else {
			m.Msg = I18nWeb(c, "somethingWentWrong")
			logger.Warningf("%s %s %s", ctx, m.Msg, fail)
		}
	}
	c.JSON(http.StatusOK, m)
}

// pendingNodeObj returns a response object flagging that the save committed
// locally but a backing node was offline/disabled, so the change will be
// mirrored to the node once it reconnects. Returns nil when nothing is pending.
func pendingNodeObj(pending bool) any {
	if pending {
		return gin.H{"nodePending": true}
	}
	return nil
}

// pureJsonMsg sends a pure JSON message response with custom status code.
func pureJsonMsg(c *gin.Context, statusCode int, success bool, msg string) {
	c.JSON(statusCode, entity.Msg{
		Success: success,
		Msg:     msg,
	})
}

// isAjax checks if the request is an AJAX request.
func isAjax(c *gin.Context) bool {
	return c.GetHeader("X-Requested-With") == "XMLHttpRequest"
}
