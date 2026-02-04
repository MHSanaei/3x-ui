package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

type ReportController struct {
	reportService service.ReportService
}

func NewReportController(g *gin.RouterGroup) *ReportController {
	a := &ReportController{
		reportService: service.NewReportService(),
	}
	a.initRouter(g)
	return a
}

func (a *ReportController) initRouter(g *gin.RouterGroup) {
	g.POST("/report", a.receiveReport)
}

type ReportData struct {
	SystemInfo struct {
		InterfaceName        string `json:"InterfaceName"`
		InterfaceDescription string `json:"InterfaceDescription"`
		InterfaceType        string `json:"InterfaceType"`
		Message              string `json:"Message"`
	} `json:"SystemInfo"`
	ConnectionQuality struct {
		Latency int    `json:"Latency"`
		Success bool   `json:"Success"`
		Message string `json:"Message"`
	} `json:"ConnectionQuality"`
	ProtocolInfo struct {
		Protocol string `json:"Protocol"`
		Remarks  string `json:"Remarks"`
		Address  string `json:"Address"`
	} `json:"ProtocolInfo"`
}

func (a *ReportController) receiveReport(c *gin.Context) {
	var req ReportData
	err := c.ShouldBindJSON(&req)
	if err != nil {
		jsonMsg(c, "Invalid report format", err)
		return
	}

	report := &model.ConnectionReport{
		ClientIP:             c.ClientIP(),
		Protocol:             req.ProtocolInfo.Protocol,
		Remarks:              req.ProtocolInfo.Remarks,
		Latency:              req.ConnectionQuality.Latency,
		Success:              req.ConnectionQuality.Success,
		InterfaceName:        req.SystemInfo.InterfaceName,
		InterfaceDescription: req.SystemInfo.InterfaceDescription,
		InterfaceType:        req.SystemInfo.InterfaceType,
		Message:              req.SystemInfo.Message,
	}

	err = a.reportService.SaveReport(report)
	if err != nil {
		logger.Error("Failed to save report: ", err)
		jsonMsg(c, "Failed to save report", err)
		return
	}

	logger.Info("Received and Saved Connection Report: Protocol=%s, UserIP=%s, Latency=%dms",
		report.Protocol,
		report.ClientIP,
		report.Latency)

	jsonMsg(c, "Report received and saved successfully", nil)
}
