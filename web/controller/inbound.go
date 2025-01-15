package controller

import (
    "errors"
	"encoding/json"
	"fmt"
	"strconv"
	"x-ui/database/model"
	"x-ui/web/service"
	"x-ui/web/session"

	"github.com/gin-gonic/gin"
)

type InboundController struct {
	inboundService service.InboundService
	xrayService    service.XrayService
}

func NewInboundController(g *gin.RouterGroup) *InboundController {
	a := &InboundController{}
	a.initRouter(g)
	return a
}

func (a *InboundController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/inbound")

	g.POST("/list", a.getInbounds)
	g.POST("/add", a.addInbound)
	g.POST("/del/:id", a.delInbound)
	g.POST("/update/:id", a.updateInbound)
	g.POST("/clientIps/:email", a.getClientIps)
	g.POST("/clearClientIps/:email", a.clearClientIps)
	g.POST("/addClient", a.addInboundClient)
	g.POST("/addGroupClient", a.addGroupInboundClient)
	g.POST("/:id/delClient/:clientId", a.delInboundClient)
	g.POST("/delGroupClients", a.delGroupClients)
	g.POST("/updateClient/:clientId", a.updateInboundClient)
	g.POST("/updateClients", a.updateGroupInboundClient)
	g.POST("/:id/resetClientTraffic/:email", a.resetClientTraffic)
	g.POST("/resetGroupClientTraffic", a.resetGroupClientTraffic)
	g.POST("/resetAllTraffics", a.resetAllTraffics)
	g.POST("/resetAllClientTraffics/:id", a.resetAllClientTraffics)
	g.POST("/delDepletedClients/:id", a.delDepletedClients)
	g.POST("/import", a.importInbound)
	g.POST("/onlines", a.onlines)
}

func (a *InboundController) getInbounds(c *gin.Context) {
	user := session.GetLoginUser(c)
	inbounds, err := a.inboundService.GetInbounds(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, inbounds, nil)
}

func (a *InboundController) getInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	inbound, err := a.inboundService.GetInbound(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, inbound, nil)
}

func (a *InboundController) getClientTraffics(c *gin.Context) {
	email := c.Param("email")
	clientTraffics, err := a.inboundService.GetClientTrafficByEmail(email)
	if err != nil {
		jsonMsg(c, "Error getting traffics", err)
		return
	}
	jsonObj(c, clientTraffics, nil)
}

func (a *InboundController) getClientTrafficsById(c *gin.Context) {
	id := c.Param("id")
	clientTraffics, err := a.inboundService.GetClientTrafficByID(id)
	if err != nil {
		jsonMsg(c, "Error getting traffics", err)
		return
	}
	jsonObj(c, clientTraffics, nil)
}

func (a *InboundController) addInbound(c *gin.Context) {
	inbound := &model.Inbound{}
	err := c.ShouldBind(inbound)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.create"), err)
		return
	}
	user := session.GetLoginUser(c)
	inbound.UserId = user.Id
	if inbound.Listen == "" || inbound.Listen == "0.0.0.0" || inbound.Listen == "::" || inbound.Listen == "::0" {
		inbound.Tag = fmt.Sprintf("inbound-%v", inbound.Port)
	} else {
		inbound.Tag = fmt.Sprintf("inbound-%v:%v", inbound.Listen, inbound.Port)
	}

	needRestart := false
	inbound, needRestart, err = a.inboundService.AddInbound(inbound)
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.create"), inbound, err)
	if err == nil && needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *InboundController) delInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "delete"), err)
		return
	}
	needRestart := true
	needRestart, err = a.inboundService.DelInbound(id)
	jsonMsgObj(c, I18nWeb(c, "delete"), id, err)
	if err == nil && needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *InboundController) updateInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}
	inbound := &model.Inbound{
		Id: id,
	}
	err = c.ShouldBind(inbound)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}
	needRestart := true
	inbound, needRestart, err = a.inboundService.UpdateInbound(inbound)
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.update"), inbound, err)
	if err == nil && needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *InboundController) getClientIps(c *gin.Context) {
	email := c.Param("email")

	ips, err := a.inboundService.GetInboundClientIps(email)
	if err != nil || ips == "" {
		jsonObj(c, "No IP Record", nil)
		return
	}

	jsonObj(c, ips, nil)
}

func (a *InboundController) clearClientIps(c *gin.Context) {
	email := c.Param("email")

	err := a.inboundService.ClearClientIps(email)
	if err != nil {
		jsonMsg(c, "Update", err)
		return
	}
	jsonMsg(c, "Log Cleared", nil)
}

func (a *InboundController) addInboundClient(c *gin.Context) {
	data := &model.Inbound{}
	err := c.ShouldBind(data)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}

	needRestart := true

	needRestart, err = a.inboundService.AddInboundClient(data)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	}
	jsonMsg(c, "Client(s) added", nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *InboundController) addGroupInboundClient(c *gin.Context) {
	var requestData []model.Inbound

    err := c.ShouldBindJSON(&requestData)

    if err != nil {
        jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
        return
    }

    needRestart := true

    for _, data := range requestData {

        needRestart, err = a.inboundService.AddInboundClient(&data)
        if err != nil {
            jsonMsg(c, "Something went wrong!", err)
            return
        }
    }

    jsonMsg(c, "Client(s) added", nil)
    if err == nil && needRestart {
        a.xrayService.SetToNeedRestart()
    }

}

func (a *InboundController) delInboundClient(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}
	clientId := c.Param("clientId")

	needRestart := true

	needRestart, err = a.inboundService.DelInboundClient(id, clientId)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	}
	jsonMsg(c, "Client deleted", nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *InboundController) delGroupClients(c *gin.Context) {
	var requestData []struct {
        InboundID int `json:"inboundId"`
        ClientID  string `json:"clientId"`
    }

    if err := c.ShouldBindJSON(&requestData); err != nil {
        jsonMsg(c, "Invalid request data", err)
        return
    }

    needRestart := false

    for _, req := range requestData {
        needRestartTmp, err := a.inboundService.DelInboundClient(req.InboundID, req.ClientID)
        if err != nil {
            jsonMsg(c, "Failed to delete client", err)
            return
        }

        if needRestartTmp {
            needRestart = true
        }
    }

    jsonMsg(c, "Clients deleted successfully", nil)

    if needRestart {
		a.xrayService.SetToNeedRestart()
    }
}

func (a *InboundController) updateInboundClient(c *gin.Context) {
	clientId := c.Param("clientId")

	inbound := &model.Inbound{}
	err := c.ShouldBind(inbound)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}

	needRestart := true

	needRestart, err = a.inboundService.UpdateInboundClient(inbound, clientId)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	}
	jsonMsg(c, "Client updated", nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *InboundController) updateGroupInboundClient(c *gin.Context) {
    var requestData []map[string]interface{}

    if err := c.ShouldBindJSON(&requestData); err != nil {
        jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
        return
    }

    needRestart := false

    for _, item := range requestData {

        inboundMap, ok := item["inbound"].(map[string]interface{})
        if !ok {
            jsonMsg(c, "Something went wrong!", errors.New("Failed to convert 'inbound' to map"))
            return
        }

        clientId, ok := item["clientId"].(string)
        if !ok {
            jsonMsg(c, "Something went wrong!", errors.New("Failed to convert 'clientId' to string"))
            return
        }

        inboundJSON, err := json.Marshal(inboundMap)
        if err != nil {
            jsonMsg(c, "Something went wrong!", err)
            return
        }

        var inboundModel model.Inbound
        if err := json.Unmarshal(inboundJSON, &inboundModel); err != nil {
            jsonMsg(c, "Something went wrong!", err)
            return
        }

        if restart, err := a.inboundService.UpdateInboundClient(&inboundModel, clientId); err != nil {
            jsonMsg(c, "Something went wrong!", err)
            return
        } else {
            needRestart = needRestart || restart
        }
    }

    jsonMsg(c, "Client updated", nil)
    if needRestart {
        a.xrayService.SetToNeedRestart()
    }
}

func (a *InboundController) resetClientTraffic(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}
	email := c.Param("email")

	needRestart, err := a.inboundService.ResetClientTraffic(id, email)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	}
	jsonMsg(c, "Traffic has been reset", nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *InboundController) resetGroupClientTraffic(c *gin.Context) {
	var requestData []struct {
		InboundID int    `json:"inboundId"` // Map JSON "inboundId" to struct field "InboundID"
		Email     string `json:"email"`    // Map JSON "email" to struct field "Email"
	}

	// Parse JSON body directly using ShouldBindJSON
	if err := c.ShouldBindJSON(&requestData); err != nil {
		jsonMsg(c, "Invalid request data", err)
		return
	}

	needRestart := false

	// Process each request data
	for _, req := range requestData {
		needRestartTmp, err := a.inboundService.ResetClientTraffic(req.InboundID, req.Email)
		if err != nil {
			jsonMsg(c, "Failed to reset client traffic", err)
			return
		}

		// If any request requires a restart, set needRestart to true
		if needRestartTmp {
			needRestart = true
		}
	}

	// Send response back to the client
	jsonMsg(c, "Traffic reset for all clients", nil)

	// Restart the service if required
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
}


func (a *InboundController) resetAllTraffics(c *gin.Context) {
	err := a.inboundService.ResetAllTraffics()
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	} else {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, "all traffic has been reset", nil)
}

func (a *InboundController) resetAllClientTraffics(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}

	err = a.inboundService.ResetAllClientTraffics(id)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	} else {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, "All traffic from the client has been reset.", nil)
}

func (a *InboundController) importInbound(c *gin.Context) {
	inbound := &model.Inbound{}
	err := json.Unmarshal([]byte(c.PostForm("data")), inbound)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	}
	user := session.GetLoginUser(c)
	inbound.Id = 0
	inbound.UserId = user.Id
	if inbound.Listen == "" || inbound.Listen == "0.0.0.0" || inbound.Listen == "::" || inbound.Listen == "::0" {
		inbound.Tag = fmt.Sprintf("inbound-%v", inbound.Port)
	} else {
		inbound.Tag = fmt.Sprintf("inbound-%v:%v", inbound.Listen, inbound.Port)
	}

	for index := range inbound.ClientStats {
		inbound.ClientStats[index].Id = 0
		inbound.ClientStats[index].Enable = true
	}

	needRestart := false
	inbound, needRestart, err = a.inboundService.AddInbound(inbound)
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.create"), inbound, err)
	if err == nil && needRestart {
		a.xrayService.SetToNeedRestart()
	}
}

func (a *InboundController) delDepletedClients(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.update"), err)
		return
	}
	err = a.inboundService.DelDepletedClients(id)
	if err != nil {
		jsonMsg(c, "Something went wrong!", err)
		return
	}
	jsonMsg(c, "All depleted clients are deleted", nil)
}

func (a *InboundController) onlines(c *gin.Context) {
	jsonObj(c, a.inboundService.GetOnlineClients(), nil)
}
