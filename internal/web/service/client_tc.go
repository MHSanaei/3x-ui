package service

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/tc"
)

func applyClientTCLimit(client model.Client) {
	if err := tc.ApplyClientLimit(client); err != nil {
		logger.Warningf("[TC] apply client speed limit for %q failed: %v", client.Email, err)
	}
}

func removeClientTCLimit(email string) {
	if err := tc.RemoveClientLimitByEmail(email); err != nil {
		logger.Warningf("[TC] remove client speed limit for %q failed: %v", email, err)
	}
}
