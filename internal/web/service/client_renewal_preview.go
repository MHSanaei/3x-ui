package service

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type ClientRenewalPreviewRequest struct {
	ExpiryTime   int64 `json:"expiryTime" example:"1893456000000"`
	Reset        int   `json:"reset" example:"0"`
	ResetDay     int   `json:"resetDay" example:"1"`
	ResetWeekday int   `json:"resetWeekday" example:"0"`
	ResetMax     int   `json:"resetMax" example:"0"`
	ResetCount   int   `json:"resetCount" example:"0"`
}

type ClientRenewalPreview struct {
	TimeZone            string `json:"timeZone" example:"UTC"`
	RenewAt             string `json:"renewAt" example:"2030-01-01T00:00:00Z"`
	ValidThrough        string `json:"validThrough" example:"2029-12-31T23:59:59Z"`
	NextExpiry          string `json:"nextExpiry" example:"2030-02-01T00:00:00Z"`
	SuggestedExpiryTime int64  `json:"suggestedExpiryTime" example:"1893456000000"`
	SuggestedExpiry     string `json:"suggestedExpiry" example:"2030-01-01T00:00:00Z"`
	Renewals            int    `json:"renewals" example:"1"`
	CanRenew            bool   `json:"canRenew" example:"true"`
	DelayedStart        bool   `json:"delayedStart" example:"false"`
}

func (s *ClientService) PreviewRenewal(request ClientRenewalPreviewRequest, settings *SettingService) (*ClientRenewalPreview, error) {
	client := model.Client{Reset: request.Reset, ResetDay: request.ResetDay, ResetWeekday: request.ResetWeekday}
	if err := validateClientRenewal(client); err != nil {
		return nil, err
	}
	if err := validateClientResetMax(request.ResetMax); err != nil {
		return nil, err
	}
	if request.ResetCount < 0 || request.Reset < 0 {
		return nil, common.NewError("renewal preview reset and resetCount must not be negative")
	}
	loc, err := settings.GetTimeLocation()
	if err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	preview := &ClientRenewalPreview{TimeZone: loc.String(), DelayedStart: request.ExpiryTime < 0}
	if request.ResetDay > 0 || request.ResetWeekday > 0 {
		preview.SuggestedExpiryTime = nextClientRenewal(now, request.Reset, request.ResetDay, request.ResetWeekday, loc)
		if preview.SuggestedExpiryTime <= now {
			return nil, common.NewError("calendar renewal could not find a future expiry")
		}
		preview.SuggestedExpiry = time.UnixMilli(preview.SuggestedExpiryTime).In(loc).Format(time.RFC3339)
	}
	if request.ExpiryTime <= 0 || (request.Reset <= 0 && request.ResetDay <= 0 && request.ResetWeekday <= 0) {
		return preview, nil
	}
	at := canonicalRenewalExpiry(request.ExpiryTime, request.Reset, request.ResetDay, request.ResetWeekday, loc)
	preview.RenewAt = time.UnixMilli(at).In(loc).Format(time.RFC3339Nano)
	// Billing alignment does not rewrite the stored exclusive deadline; the
	// preview must not promise an extra second before the first renewal.
	preview.ValidThrough = time.UnixMilli(request.ExpiryTime - 1).In(loc).Format(time.RFC3339)
	traffic := &xray.ClientTraffic{
		ExpiryTime: request.ExpiryTime, Reset: request.Reset, ResetDay: request.ResetDay,
		ResetWeekday: request.ResetWeekday, ResetMax: request.ResetMax, ResetCount: request.ResetCount,
	}
	expiry, renewals := catchUpClientRenewal(traffic, max(now, at+1), loc)
	if renewals == 0 && request.ResetWeekday > 0 && (request.ResetMax == 0 || request.ResetCount < request.ResetMax) {
		return nil, common.NewError("calendar renewal could not find a future expiry")
	}
	preview.Renewals = renewals
	preview.CanRenew = renewals > 0 && expiry > max(now, at)
	if renewals > 0 {
		preview.NextExpiry = time.UnixMilli(expiry).In(loc).Format(time.RFC3339Nano)
	}
	return preview, nil
}
