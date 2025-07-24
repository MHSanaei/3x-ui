package middleware

import (
	"net/http"
	"x-ui/web/service"
	"github.com/gin-gonic/gin"
)

func ApiAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("Api-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key is required"})
			c.Abort()
			return
		}

		settingService := service.SettingService{}
		panelAPIKey, err := settingService.GetAPIKey()
		if err != nil || panelAPIKey == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "API key not configured on the panel"})
			c.Abort()
			return
		}

		if apiKey != panelAPIKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		c.Next()
	}
}
