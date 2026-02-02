package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// AIService provides Gemini AI integration for intelligent features
type AIService struct {
	client       *genai.Client
	model        *genai.GenerativeModel
	settingService SettingService
	inboundService InboundService
	serverService  ServerService
	
	// Cache and rate limiting
	cache          sync.Map // Cache for recent AI responses
	rateLimiter    map[int64]*rateLimitState
	rateLimiterMu  sync.RWMutex
	
	// Configuration
	enabled        bool
	apiKey         string
	maxTokens      int32
	temperature    float32
	cacheDuration  time.Duration
}

type rateLimitState struct {
	requests  int
	resetTime time.Time
	mu        sync.Mutex
}

type cacheEntry struct {
	response  string
	timestamp time.Time
}

// AIIntent represents the detected user intent from natural language
type AIIntent struct {
	Action      string                 `json:"action"`       // status, usage, inbound_list, client_add, etc.
	Parameters  map[string]interface{} `json:"parameters"`   // Extracted parameters
	Confidence  float64                `json:"confidence"`   // Confidence score 0-1
	NeedsAction bool                   `json:"needs_action"` // Whether this requires bot action
	Response    string                 `json:"response"`     // AI-generated response text
}

const (
	// Rate limiting
	maxRequestsPerMinute = 20
	maxRequestsPerHour   = 100
	
	// Cache settings
	defaultCacheDuration = 5 * time.Minute
	
	// AI Model settings
	defaultModel       = "gemini-1.5-flash"
	defaultMaxTokens   = 1024
	defaultTemperature = 0.7
	
	// System prompt for the AI
	systemPrompt = `You are an intelligent assistant for a VPN/Proxy management panel called 3X-UI.

Your role is to understand user commands in natural language and help manage their VPN server.

Available actions:
- server_status: Show CPU, memory, disk usage, uptime, Xray status
- server_usage: Display traffic statistics (total/upload/download)
- inbound_list: List all inbound configurations
- inbound_info: Get details about a specific inbound (by ID or remark)
- client_list: List clients for an inbound
- client_add: Add a new client to an inbound
- client_reset: Reset client traffic
- client_delete: Delete a client
- settings_backup: Create a backup
- settings_restore: Restore from backup
- help: Show available commands

When analyzing user messages:
1. Detect the intent/action they want to perform
2. Extract relevant parameters (inbound ID, client email, etc.)
3. Provide a confidence score (0-1) for your interpretation
4. Generate a helpful response

If the user's request is unclear or ambiguous, ask clarifying questions.
Always be concise, professional, and helpful.

Respond ONLY with valid JSON in this exact format:
{
  "action": "detected_action",
  "parameters": {"key": "value"},
  "confidence": 0.95,
  "needs_action": true,
  "response": "Your helpful response text"
}`
)

// NewAIService initializes the AI service with Gemini API
func NewAIService() *AIService {
	service := &AIService{
		rateLimiter:   make(map[int64]*rateLimitState),
		maxTokens:     defaultMaxTokens,
		temperature:   defaultTemperature,
		cacheDuration: defaultCacheDuration,
	}
	
	// Load settings from database
	if err := service.loadSettings(); err != nil {
		logger.Warning("AI Service: Failed to load settings:", err)
		return service
	}
	
	// Initialize client if enabled
	if service.enabled && service.apiKey != "" {
		if err := service.initClient(); err != nil {
			logger.Warning("AI Service: Failed to initialize Gemini client:", err)
			service.enabled = false
		}
	}
	
	return service
}

// loadSettings loads AI configuration from database
func (s *AIService) loadSettings() error {
	db := database.GetDB()
	
	// Check if AI is enabled
	enabledStr, err := s.settingService.GetAISetting("AIEnabled")
	if err == nil && enabledStr == "true" {
		s.enabled = true
	}
	
	// Load API key
	apiKey, err := s.settingService.GetAISetting("AIApiKey")
	if err == nil && apiKey != "" {
		s.apiKey = apiKey
	}
	
	// Load optional settings
	if maxTokensStr, err := s.settingService.GetAISetting("AIMaxTokens"); err == nil {
		var maxTokens int
		if err := json.Unmarshal([]byte(maxTokensStr), &maxTokens); err == nil {
			s.maxTokens = int32(maxTokens)
		}
	}
	
	if tempStr, err := s.settingService.GetAISetting("AITemperature"); err == nil {
		var temp float64
		if err := json.Unmarshal([]byte(tempStr), &temp); err == nil {
			s.temperature = float32(temp)
		}
	}
	
	logger.Debug("AI Service settings loaded - Enabled:", s.enabled, "API Key present:", s.apiKey != "")
	
	return db.Error
}

// initClient initializes the Gemini AI client
func (s *AIService) initClient() error {
	ctx := context.Background()
	
	client, err := genai.NewClient(ctx, option.WithAPIKey(s.apiKey))
	if err != nil {
		return fmt.Errorf("failed to create Gemini client: %w", err)
	}
	
	s.client = client
	s.model = client.GenerativeModel(defaultModel)
	
	// Configure model parameters
	s.model.SetMaxOutputTokens(s.maxTokens)
	s.model.SetTemperature(s.temperature)
	s.model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}
	
	// Configure safety settings to be less restrictive for technical content
	s.model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockMediumAndAbove,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockMediumAndAbove,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockMediumAndAbove,
		},
	}
	
	logger.Info("AI Service: Gemini client initialized successfully")
	return nil
}

// IsEnabled checks if AI service is currently enabled
func (s *AIService) IsEnabled() bool {
	return s.enabled && s.client != nil
}

// ProcessMessage processes a natural language message and returns detected intent
func (s *AIService) ProcessMessage(ctx context.Context, userID int64, message string) (*AIIntent, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("AI service is not enabled")
	}
	
	// Check rate limiting
	if !s.checkRateLimit(userID) {
		return nil, fmt.Errorf("rate limit exceeded, please try again later")
	}
	
	// Check cache first
	cacheKey := fmt.Sprintf("%d:%s", userID, strings.ToLower(strings.TrimSpace(message)))
	if cached, ok := s.cache.Load(cacheKey); ok {
		entry := cached.(cacheEntry)
		if time.Since(entry.timestamp) < s.cacheDuration {
			logger.Debug("AI Service: Cache hit for user", userID)
			var intent AIIntent
			if err := json.Unmarshal([]byte(entry.response), &intent); err == nil {
				return &intent, nil
			}
		}
	}
	
	// Generate AI response
	intent, err := s.generateIntent(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("failed to generate intent: %w", err)
	}
	
	// Cache the response
	responseJSON, _ := json.Marshal(intent)
	s.cache.Store(cacheKey, cacheEntry{
		response:  string(responseJSON),
		timestamp: time.Now(),
	})
	
	logger.Debug("AI Service: Processed message for user", userID, "- Action:", intent.Action, "Confidence:", intent.Confidence)
	
	return intent, nil
}

// generateIntent calls Gemini API to analyze the message
func (s *AIService) generateIntent(ctx context.Context, message string) (*AIIntent, error) {
	// Create prompt with user message
	prompt := fmt.Sprintf("User message: %s\n\nAnalyze this message and respond with the JSON format specified in the system prompt.", message)
	
	// Set timeout for API call
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	
	// Generate response
	resp, err := s.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("Gemini API error: %w", err)
	}
	
	// Extract text response
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini API")
	}
	
	responseText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	
	// Parse JSON response
	intent, err := s.parseIntentResponse(responseText)
	if err != nil {
		// If parsing fails, try to extract JSON from markdown code block
		if cleaned := extractJSONFromMarkdown(responseText); cleaned != "" {
			intent, err = s.parseIntentResponse(cleaned)
		}
		
		if err != nil {
			logger.Warning("AI Service: Failed to parse response:", err, "Raw:", responseText)
			// Return a fallback intent
			return &AIIntent{
				Action:      "unknown",
				Parameters:  make(map[string]interface{}),
				Confidence:  0.0,
				NeedsAction: false,
				Response:    "I couldn't understand your request. Please try rephrasing or use /help to see available commands.",
			}, nil
		}
	}
	
	return intent, nil
}

// parseIntentResponse parses the JSON response from Gemini
func (s *AIService) parseIntentResponse(responseText string) (*AIIntent, error) {
	var intent AIIntent
	
	// Try to parse as JSON
	if err := json.Unmarshal([]byte(responseText), &intent); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}
	
	// Validate required fields
	if intent.Action == "" {
		intent.Action = "unknown"
	}
	if intent.Parameters == nil {
		intent.Parameters = make(map[string]interface{})
	}
	if intent.Response == "" {
		intent.Response = "Processing your request..."
	}
	
	return &intent, nil
}

// checkRateLimit checks if user has exceeded rate limits
func (s *AIService) checkRateLimit(userID int64) bool {
	now := time.Now()
	
	s.rateLimiterMu.Lock()
	defer s.rateLimiterMu.Unlock()
	
	state, exists := s.rateLimiter[userID]
	if !exists {
		state = &rateLimitState{
			requests:  1,
			resetTime: now.Add(time.Minute),
		}
		s.rateLimiter[userID] = state
		return true
	}
	
	state.mu.Lock()
	defer state.mu.Unlock()
	
	// Reset if time window passed
	if now.After(state.resetTime) {
		state.requests = 1
		state.resetTime = now.Add(time.Minute)
		return true
	}
	
	// Check limit
	if state.requests >= maxRequestsPerMinute {
		return false
	}
	
	state.requests++
	return true
}

// GetContextForUser generates context information to enhance AI responses
func (s *AIService) GetContextForUser(userID int64) string {
	var context strings.Builder
	
	// Add server status
	if serverInfo, err := s.serverService.GetStatus(true); err == nil {
		context.WriteString(fmt.Sprintf("Server CPU: %.1f%%, Memory: %.1f%%, ", 
			serverInfo.Cpu, serverInfo.Mem))
	}
	
	// Add inbound count
	if db := database.GetDB(); db != nil {
		var count int64
		db.Model(&struct{}{}).Count(&count)
		context.WriteString(fmt.Sprintf("Total inbounds: %d. ", count))
	}
	
	return context.String()
}

// Close gracefully shuts down the AI service
func (s *AIService) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// extractJSONFromMarkdown extracts JSON from markdown code blocks
func extractJSONFromMarkdown(text string) string {
	// Try to find JSON in ```json or ``` blocks
	patterns := []string{
		"```json\n(.+?)\n```",
		"```\n(.+?)\n```",
	}
	
	for _, pattern := range patterns {
		if idx := strings.Index(text, "```"); idx != -1 {
			// Find closing ```
			if endIdx := strings.Index(text[idx+3:], "```"); endIdx != -1 {
				extracted := text[idx+3 : idx+3+endIdx]
				// Remove "json" if present at start
				extracted = strings.TrimPrefix(extracted, "json")
				extracted = strings.TrimSpace(extracted)
				return extracted
			}
		}
	}
	
	// Try to find JSON by looking for { and }
	if start := strings.Index(text, "{"); start != -1 {
		if end := strings.LastIndex(text, "}"); end != -1 && end > start {
			return text[start : end+1]
		}
	}
	
	return ""
}
