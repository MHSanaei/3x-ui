package sub

import (
	"encoding/base64"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"

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
	jsonNoise string,
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
		subJsonService: NewSubJsonService(jsonFragment, jsonNoise, jsonMux, jsonRules, sub),
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
	var host string
	if h, err := getHostFromXFH(c.GetHeader("X-Forwarded-Host")); err == nil {
		host = h
	}
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
		resultSlice := strings.Split(strings.TrimSpace(result), "\n")

		// Add headers
		c.Writer.Header().Set("Subscription-Userinfo", header)
		c.Writer.Header().Set("Profile-Update-Interval", a.updateInterval)
		c.Writer.Header().Set("Profile-Title", subId)

		acceptHeader := c.GetHeader("Accept")
		headerMap := parseHeaderString(header)
		expireValue := headerMap["expire"]
		upValue := formatBytes(headerMap["upload"], 2)
		downValue := formatBytes(headerMap["download"], 2)
		totalValue := formatBytes(headerMap["total"], 2)

		currentURL := "https://" + c.Request.Host + c.Request.RequestURI

		if strings.Contains(acceptHeader, "text/html") {
			if a.subEncrypt {
				c.String(200, base64.StdEncoding.EncodeToString([]byte(result)))
			} else {
				c.HTML(200, "sub.html", gin.H{
					"result":       resultSlice,
					"total":        totalValue,
					"expire":       expireValue,
					"upload":       upValue,
					"download":     downValue,
					"totalByte":    headerMap["total"],
					"uploadByte":   headerMap["upload"],
					"downloadByte": headerMap["download"],
					"sId":          subId,
					"subUrl":       currentURL,
				})
			}
		} else {
			if a.subEncrypt {
				c.String(200, base64.StdEncoding.EncodeToString([]byte(result)))
			} else {
				c.String(200, result)
			}
		}
	}
}

func (a *SUBController) subJsons(c *gin.Context) {
	subId := c.Param("subid")
	var host string
	if h, err := getHostFromXFH(c.GetHeader("X-Forwarded-Host")); err == nil {
		host = h
	}
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

func getHostFromXFH(s string) (string, error) {
	if strings.Contains(s, ":") {
		realHost, _, err := net.SplitHostPort(s)
		if err != nil {
			return "", err
		}
		return realHost, nil
	}
	return s, nil
}

func parseHeaderString(header string) map[string]string {
	headerMap := make(map[string]string)
	pairs := strings.Split(header, ";")
	for _, pair := range pairs {
		kv := strings.Split(strings.TrimSpace(pair), "=")
		if len(kv) == 2 {
			headerMap[kv[0]] = kv[1]
		}
	}
	return headerMap
}

func formatBytes(sizeStr string, precision int) string {
	// Convert the string input to a float64
	size, _ := strconv.ParseFloat(sizeStr, 64)

	if size == 0 {
		return "0 B"
	}

	// Calculate base and suffix
	base := math.Log(size) / math.Log(1024)
	suffixes := []string{"B", "K", "M", "G", "T"}

	value := math.Pow(1024, base-math.Floor(base))
	return fmt.Sprintf("%.*f %s", precision, value, suffixes[int(math.Floor(base))])
}
