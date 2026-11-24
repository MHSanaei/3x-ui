package controller

import (
	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func (a *ClientController) wireguardDevices(c *gin.Context) {
	devices, err := a.clientService.WireguardDevices(c.Query("email"))
	jsonObj(c, devices, err)
}

func (a *ClientController) finishWireguardDeviceWrite(c *gin.Context, err error) {
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	// Not left to the 30 s reconcile tick: until then a disabled or deleted device
	// kept its tunnel (measured 12-24 s on a running panel).
	a.xrayService.ReconcileNow()
	notifyClientsChanged()
	a.wireguardDevices(c)
}

func (a *ClientController) addWireguardDevice(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonObj(c, nil, err)
		return
	}
	a.finishWireguardDeviceWrite(c, a.clientService.AddWireguardDevice(c.Query("email"), body.Name))
}

func (a *ClientController) updateWireguardDevice(c *gin.Context) {
	var patch service.WireguardDevicePatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		jsonObj(c, nil, err)
		return
	}
	a.finishWireguardDeviceWrite(c, a.clientService.UpdateWireguardDevice(c.Query("email"), c.Param("deviceId"), patch))
}

func (a *ClientController) deleteWireguardDevice(c *gin.Context) {
	a.finishWireguardDeviceWrite(c, a.clientService.DeleteWireguardDevice(c.Query("email"), c.Param("deviceId")))
}

func (a *ClientController) setWireguardDeviceLimit(c *gin.Context) {
	var body struct {
		Limit int `json:"limit"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonObj(c, nil, err)
		return
	}
	a.finishWireguardDeviceWrite(c, a.clientService.SetWireguardDeviceLimit(c.Query("email"), body.Limit))
}
