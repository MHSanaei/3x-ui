package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/portforward"
)

// GenerateHysteriaPortForwardRules generates firewall rules for Hysteria UDP port hopping
func (a *InboundController) GenerateHysteriaPortForwardRules(c *gin.Context) {
	var req struct {
		BasePort     int    `json:"basePort" binding:"required,min=1,max=65535"`
		PortRange    string `json:"portRange" binding:"required"`
		FirewallType string `json:"firewallType"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	fwType := portforward.FirewallType(req.FirewallType)
	if fwType == "" {
		fwType = portforward.FirewallIptables
	}

	generator, err := portforward.NewGenerator(req.BasePort, req.PortRange, fwType)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ruleSet := generator.Generate()
	c.JSON(200, gin.H{
		"firewallType": string(ruleSet.FirewallType),
		"rules":        ruleSet.ToStringSlice(),
	})
}
