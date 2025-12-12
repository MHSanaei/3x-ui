package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
)

// OnboardingService handles automated client onboarding
type OnboardingService struct {
	inboundService InboundService
	xrayService    XrayService
	tgbotService   Tgbot
}

// OnboardingRequest represents a client onboarding request
type OnboardingRequest struct {
	Email      string `json:"email"`
	InboundTag string `json:"inbound_tag"`
	TotalGB    int64  `json:"total_gb"`
	ExpiryDays int    `json:"expiry_days"`
	LimitIP    int    `json:"limit_ip"`
	Protocol   string `json:"protocol"`
	SendConfig bool   `json:"send_config"`
	SendMethod string `json:"send_method"` // email, telegram, webhook
}

// OnboardClient creates a new client automatically
func (s *OnboardingService) OnboardClient(req OnboardingRequest) (*model.Client, error) {
	// Validate request
	if req.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if req.InboundTag == "" {
		return nil, fmt.Errorf("inbound tag is required")
	}

	// Get inbound by tag
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, fmt.Errorf("failed to get inbounds: %w", err)
	}

	var targetInbound *model.Inbound
	for i := range inbounds {
		if inbounds[i].Tag == req.InboundTag {
			targetInbound = inbounds[i]
			break
		}
	}

	if targetInbound == nil {
		return nil, fmt.Errorf("inbound with tag %s not found", req.InboundTag)
	}

	// Check if client already exists
	clients, _ := s.inboundService.GetClients(targetInbound)
	for _, c := range clients {
		if c.Email == req.Email {
			return nil, fmt.Errorf("client with email %s already exists", req.Email)
		}
	}

	// Create new client
	newClient := model.Client{
		Email:   req.Email,
		Enable:  true,
		LimitIP: req.LimitIP,
		TotalGB: req.TotalGB,
	}

	if req.ExpiryDays > 0 {
		newClient.ExpiryTime = time.Now().Add(time.Duration(req.ExpiryDays) * 24 * time.Hour).UnixMilli()
	}

	// Generate credentials based on protocol
	switch targetInbound.Protocol {
	case model.Trojan, model.Shadowsocks:
		newClient.Password = uuid.NewString()
	default:
		newClient.ID = uuid.NewString()
	}

	// Add client to inbound
	clientJSON, err := json.Marshal(newClient)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal client: %w", err)
	}

	payload := &model.Inbound{
		Id:       targetInbound.Id,
		Settings: fmt.Sprintf(`{"clients":[%s]}`, string(clientJSON)),
	}

	_, err = s.inboundService.AddInboundClient(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to add client to inbound: %w", err)
	}

	// Send configuration if requested
	if req.SendConfig {
		s.sendClientConfig(req.Email, newClient, targetInbound, req.SendMethod)
	}

	logger.Infof("Client %s onboarded successfully", req.Email)
	return &newClient, nil
}

// sendClientConfig sends client configuration via specified method
func (s *OnboardingService) sendClientConfig(email string, client model.Client, inbound *model.Inbound, method string) {
	config := s.generateClientConfig(client, inbound)

	switch method {
	case "telegram":
		// Send via Telegram bot (implement when Tgbot service has SendMessage)
		logger.Infof("New client configuration for %s:\n%s", email, config)
	case "email":
		// Send via email (implement email service)
		logger.Info("Email sending not implemented yet")
	case "webhook":
		// Send via webhook
		logger.Info("Webhook sending not implemented yet")
	}
}

// generateClientConfig generates client configuration string
func (s *OnboardingService) generateClientConfig(client model.Client, inbound *model.Inbound) string {
	// Generate configuration based on protocol
	// This is simplified - in production, generate full Xray config
	return fmt.Sprintf("Email: %s\nProtocol: %s\nID: %s", client.Email, inbound.Protocol, client.ID)
}

// ProcessWebhook processes incoming webhook for client creation
func (s *OnboardingService) ProcessWebhook(webhookData map[string]interface{}) error {
	// Parse webhook data
	email, ok := webhookData["email"].(string)
	if !ok {
		return fmt.Errorf("email is required")
	}

	req := OnboardingRequest{
		Email:      email,
		InboundTag: getString(webhookData, "inbound_tag", "default"),
		TotalGB:    getInt64(webhookData, "total_gb", 100),
		ExpiryDays: getInt(webhookData, "expiry_days", 30),
		LimitIP:    getInt(webhookData, "limit_ip", 0),
		SendConfig: getBool(webhookData, "send_config", true),
		SendMethod: getString(webhookData, "send_method", "telegram"),
	}

	_, err := s.OnboardClient(req)
	return err
}

func getString(m map[string]interface{}, key, defaultValue string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultValue
}

func getInt(m map[string]interface{}, key string, defaultValue int) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return defaultValue
}

func getInt64(m map[string]interface{}, key string, defaultValue int64) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return defaultValue
}

func getBool(m map[string]interface{}, key string, defaultValue bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return defaultValue
}
