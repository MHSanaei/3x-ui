package sub

import (
	"errors"
	"net/url"
	"strings"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type subPlaceholderData struct {
	SubID   string
	Context remarkContext
	HasCtx  bool
	Escape  bool
}

type renderedSubMetadata struct {
	Title      string
	SupportURL string
	ProfileURL string
	Announce   string
}

func renderSubPlaceholders(value string, data subPlaceholderData) string {
	if value == "" || !strings.Contains(value, "{") {
		return value
	}

	ctx := data.Context
	if !data.HasCtx {
		ctx = remarkContext{
			client: model.Client{
				SubID: data.SubID,
			},
		}
	}
	if ctx.client.SubID == "" {
		ctx.client.SubID = data.SubID
	}
	return strings.TrimSpace(expandSubMetadataVars(value, ctx, data.Escape))
}

var subMetadataTokens = map[string]bool{
	"EMAIL":       true,
	"ID":          true,
	"SHORT_ID":    true,
	"TELEGRAM_ID": true,
	"SUB_ID":      true,
}

func expandSubMetadataVars(template string, ctx remarkContext, escape bool) string {
	return remarkVarRe.ReplaceAllStringFunc(template, func(match string) string {
		token := match[2 : len(match)-2]
		if !subMetadataTokens[token] {
			return match
		}
		value := remarkVarValue(token, ctx)
		if escape {
			return url.QueryEscape(value)
		}
		return value
	})
}

func subMetadataUsesPlaceholders(values ...string) bool {
	for _, value := range values {
		if strings.Contains(value, "{") {
			return true
		}
	}
	return false
}

func (a *SUBController) metadataForSubRequest(getSubReq func() *SubService, subID string, fallbackProfileURL string) renderedSubMetadata {
	var context remarkContext
	var hasContext bool
	if subMetadataUsesPlaceholders(a.subTitle, a.subSupportUrl, a.subProfileUrl, a.subAnnounce) {
		var err error
		subReq := getSubReq()
		context, hasContext, err = subReq.subscriptionTemplateContextBySubID(subID)
		if err != nil {
			logger.Warning("sub: load template contexts for subscription metadata:", err)
		}
	}
	profileURL := a.subProfileUrl
	if profileURL == "" {
		profileURL = fallbackProfileURL
	} else {
		profileURL = renderSubPlaceholders(profileURL, subPlaceholderData{SubID: subID, Context: context, HasCtx: hasContext, Escape: true})
	}
	data := subPlaceholderData{SubID: subID, Context: context, HasCtx: hasContext}
	return renderedSubMetadata{
		Title:      renderSubPlaceholders(a.subTitle, data),
		SupportURL: renderSubPlaceholders(a.subSupportUrl, subPlaceholderData{SubID: subID, Context: context, HasCtx: hasContext, Escape: true}),
		ProfileURL: profileURL,
		Announce:   renderSubPlaceholders(a.subAnnounce, data),
	}
}

func (s *SubService) subscriptionTemplateContextBySubID(subID string) (remarkContext, bool, error) {
	if subID == "" {
		return remarkContext{}, false, nil
	}
	var rec model.ClientRecord
	err := database.GetDB().Where("sub_id = ?", subID).Order("id ASC").First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return remarkContext{}, false, nil
	}
	if err != nil {
		return remarkContext{}, false, err
	}
	return remarkContext{client: *rec.ToClient()}, true, nil
}
