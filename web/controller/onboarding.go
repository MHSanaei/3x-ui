package controller

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// OnboardingController handles client onboarding endpoints
type OnboardingController struct {
	onboardingService service.OnboardingService
}

// NewOnboardingController creates a new onboarding controller
func NewOnboardingController(g *gin.RouterGroup) *OnboardingController {
	o := &OnboardingController{
		onboardingService: service.OnboardingService{},
	}
	o.initRouter(g)
	return o
}

func (o *OnboardingController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/onboarding")
	g.POST("/client", o.onboardClient)
	g.POST("/webhook", o.processWebhook)
}

// onboardClient creates a new client automatically
func (o *OnboardingController) onboardClient(c *gin.Context) {
	var req service.OnboardingRequest
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	// Validate request
	if req.Email == "" {
		jsonMsg(c, "Email is required", errors.New("email is required"))
		return
	}
	if req.InboundTag == "" {
		jsonMsg(c, "Inbound tag is required", errors.New("inbound_tag is required"))
		return
	}
	if req.TotalGB < 0 {
		jsonMsg(c, "Total GB cannot be negative", errors.New("total_gb cannot be negative"))
		return
	}
	if req.ExpiryDays < 0 {
		jsonMsg(c, "Expiry days cannot be negative", errors.New("expiry_days cannot be negative"))
		return
	}
	if req.LimitIP < 0 {
		jsonMsg(c, "Limit IP cannot be negative", errors.New("limit_ip cannot be negative"))
		return
	}

	client, err := o.onboardingService.OnboardClient(req)
	if err != nil {
		jsonMsg(c, "Failed to onboard client", err)
		return
	}

	jsonObj(c, client, nil)
}

// processWebhook processes incoming webhook
func (o *OnboardingController) processWebhook(c *gin.Context) {
	var webhookData map[string]interface{}
	if err := c.ShouldBind(&webhookData); err != nil {
		jsonMsg(c, "Invalid request", err)
		return
	}

	err := o.onboardingService.ProcessWebhook(webhookData)
	jsonMsg(c, "Process webhook", err)
}
