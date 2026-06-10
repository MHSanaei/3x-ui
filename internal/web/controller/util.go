package controller

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

// getRemoteIp extracts the real IP address from the request headers or remote address.
func getRemoteIp(c *gin.Context) string {
	remoteIP, ok := extractTrustedIP(c.Request.RemoteAddr)
	if !ok {
		return "unknown"
	}

	if isTrustedProxy(remoteIP) {
		if ip, ok := extractTrustedIP(c.GetHeader("X-Real-IP")); ok {
			return ip
		}

		if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
			for part := range strings.SplitSeq(xff, ",") {
				if ip, ok := extractTrustedIP(part); ok {
					return ip
				}
			}
		}
	}

	return remoteIP
}

func isTrustedForwardedRequest(c *gin.Context) bool {
	remoteIP, ok := extractTrustedIP(c.Request.RemoteAddr)
	return ok && isTrustedProxy(remoteIP)
}

func isTrustedProxy(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	trusted := trustedProxyCIDRs()
	for value := range strings.SplitSeq(trusted, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(value); err == nil {
			if prefix.Contains(addr) {
				return true
			}
			continue
		}
		if proxyIP, err := netip.ParseAddr(value); err == nil && proxyIP.Unmap() == addr.Unmap() {
			return true
		}
	}
	return false
}

func trustedProxyCIDRs() (trusted string) {
	trusted = "127.0.0.1/32,::1/128"
	defer func() {
		_ = recover()
	}()
	settingService := service.SettingService{}
	if value, err := settingService.GetTrustedProxyCIDRs(); err == nil && strings.TrimSpace(value) != "" {
		trusted = value
	}
	return trusted
}

func extractTrustedIP(value string) (string, bool) {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return "", false
	}

	if ip, ok := parseIPCandidate(candidate); ok {
		return ip.String(), true
	}

	if host, _, err := net.SplitHostPort(candidate); err == nil {
		if ip, ok := parseIPCandidate(host); ok {
			return ip.String(), true
		}
	}

	if strings.Count(candidate, ":") == 1 {
		if host, _, err := net.SplitHostPort(fmt.Sprintf("[%s]", candidate)); err == nil {
			if ip, ok := parseIPCandidate(host); ok {
				return ip.String(), true
			}
		}
	}

	return "", false
}

func parseIPCandidate(value string) (netip.Addr, bool) {
	ip, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
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
