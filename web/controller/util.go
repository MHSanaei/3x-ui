package controller

import (
	"net"
	"net/http"
	"strings"
	"x-ui/config"
	"x-ui/logger"
	"x-ui/web/entity"

	"github.com/gin-gonic/gin"
)

func getRemoteIp(c *gin.Context) string {
	value := c.GetHeader("X-Forwarded-For")
	if value != "" {
		ips := strings.Split(value, ",")
		return ips[0]
	} else {
		addr := c.Request.RemoteAddr
		ip, _, _ := net.SplitHostPort(addr)
		return ip
	}
}

func jsonMsg(c *gin.Context, msg string, err error) {
	jsonMsgObj(c, msg, nil, err)
}

func jsonObj(c *gin.Context, obj interface{}, err error) {
	jsonMsgObj(c, "", obj, err)
}

func jsonMsgObj(c *gin.Context, msg string, obj interface{}, err error) {
	m := entity.Msg{
		Obj: obj,
	}
	if err == nil {
		m.Success = true
		if msg != "" {
			m.Msg = msg + I18nWeb(c, "success")
		}
	} else {
		m.Success = false
		m.Msg = msg + I18nWeb(c, "fail") + ": " + err.Error()
		logger.Warning(msg+I18nWeb(c, "fail")+": ", err)
	}
	c.JSON(http.StatusOK, m)
}

func pureJsonMsg(c *gin.Context, success bool, msg string) {
	if success {
		c.JSON(http.StatusOK, entity.Msg{
			Success: true,
			Msg:     msg,
		})
	} else {
		c.JSON(http.StatusOK, entity.Msg{
			Success: false,
			Msg:     msg,
		})
	}
}

func html(c *gin.Context, name string, title string, data gin.H) {
	if data == nil {
		data = gin.H{}
	}
	data["title"] = title
	data["host"] = strings.Split(c.Request.Host, ":")[0]
	data["request_uri"] = c.Request.RequestURI
	data["base_path"] = c.GetString("base_path")
	c.HTML(http.StatusOK, name, getContext(data))
}

func getContext(h gin.H) gin.H {
	a := gin.H{
		"cur_ver": config.GetVersion(),
	}
	for key, value := range h {
		a[key] = value
	}
	return a
}

func isAjax(c *gin.Context) bool {
	return c.GetHeader("X-Requested-With") == "XMLHttpRequest"
}
