package controller

import (
	"math"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type ClientPayload struct {
	Client           *model.ClientRecord        `json:"client"`
	InboundIds       []int                      `json:"inboundIds"`
	ExternalLinks    []model.ClientExternalLink `json:"externalLinks"`
	UsedTraffic      int64                      `json:"usedTraffic" example:"0"`
	TunnelAllowedIPs map[int]string             `json:"tunnelAllowedIPs"`
}

type ClientDetail struct {
	ClientPayload
	DepletedAt     int64  `json:"depletedAt" example:"0"`
	DepletedReason string `json:"depletedReason" example:""`
}

type ClientTrafficDetail struct {
	xray.ClientTraffic
	DepletedAt     int64  `json:"depletedAt" example:"0"`
	DepletedReason string `json:"depletedReason" example:""`
}

func effectiveTrafficUsed(up, down int64) int64 {
	if down > 0 && up > math.MaxInt64-down {
		return math.MaxInt64
	}
	if down < 0 && up < math.MinInt64-down {
		return math.MinInt64
	}
	return up + down
}

// Traffic exhaustion has no historical stamp: lastOnline is an estimate, not an audit event.
func deriveClientDepletion(up, down, total, expiryTime, lastOnline, now int64) (int64, string) {
	traffic := total > 0 && effectiveTrafficUsed(up, down) >= total
	expiry := expiryTime > 0 && now >= expiryTime
	switch {
	case traffic && expiry:
		if lastOnline > 0 {
			return min(lastOnline, expiryTime), "both"
		}
		return expiryTime, "both"
	case traffic:
		return lastOnline, "traffic"
	case expiry:
		return expiryTime, "expiry"
	default:
		return 0, ""
	}
}
