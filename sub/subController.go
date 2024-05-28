package sub

import (
	"encoding/base64"
	"net"

	"github.com/gin-gonic/gin"
)

type SUBController struct {
	subPath        string
	subJsonPath    string
	subEncrypt     bool
	updateInterval string

	subService     *SubService
	subJsonService *SubJsonService
}

func NewSUBController(
	g *gin.RouterGroup,
	subPath string,
	jsonPath string,
	encrypt bool,
	showInfo bool,
	rModel string,
	update string,
	jsonFragment string,
	jsonMux string,
	jsonRules string,
) *SUBController {
	sub := NewSubService(showInfo, rModel)
	a := &SUBController{
		subPath:        subPath,
		subJsonPath:    jsonPath,
		subEncrypt:     encrypt,
		updateInterval: update,

		subService:     sub,
		subJsonService: NewSubJsonService(jsonFragment, jsonMux, jsonRules, sub),
	}
	a.initRouter(g)
	return a
}

func (a *SUBController) initRouter(g *gin.RouterGroup) {
	gLink := g.Group(a.subPath)
	gJson := g.Group(a.subJsonPath)

	gLink.GET(":subid", a.subs)

	gJson.GET(":subid", a.subJsons)
}

func (a *SUBController) subs(c *gin.Context) {
	subId := c.Param("subid")
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.GetHeader("X-Real-IP")
	}
	if host == "" {
		var err error
		host, _, err = net.SplitHostPort(c.Request.Host)
		if err != nil {
			host = c.Request.Host
		}
	}
	subs, header, err := a.subService.GetSubs(subId, host)
	if err != nil || len(subs) == 0 {
		c.String(400, "Error!")
	} else {
		result := ""
		for _, sub := range subs {
			result += sub + "\n"
		}

		// Add headers
		c.Writer.Header().Set("Subscription-Userinfo", header)
		c.Writer.Header().Set("Profile-Update-Interval", a.updateInterval)
		c.Writer.Header().Set("Profile-Title", subId)

		if a.subEncrypt {
			c.String(200, base64.StdEncoding.EncodeToString([]byte(result)))
		} else {
			c.String(200, result)
		}
	}
}

func (a *SUBController) subJsons(c *gin.Context) {
	subId := c.Param("subid")
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.GetHeader("X-Real-IP")
	}
	if host == "" {
		var err error
		host, _, err = net.SplitHostPort(c.Request.Host)
		if err != nil {
			host = c.Request.Host
		}
	}
	jsonSub, header, err := a.subJsonService.GetJson(subId, host)
	if err != nil || len(jsonSub) == 0 {
		c.String(400, "Error!")
	} else {

		// Add headers
		c.Writer.Header().Set("Subscription-Userinfo", header)
		c.Writer.Header().Set("Profile-Update-Interval", a.updateInterval)
		c.Writer.Header().Set("Profile-Title", subId)

		c.String(200, jsonSub)
	}
}
