package service

import (
	"encoding/json"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
)

type OutboundOption struct {
	Tag      string `json:"tag"`
	Protocol string `json:"protocol,omitempty"`
	Remark   string `json:"remark,omitempty"`
	Source   string `json:"source,omitempty"`
}

func outboundOptionFromAny(raw any, source string) (OutboundOption, bool) {
	m, ok := raw.(map[string]any)
	if !ok {
		return OutboundOption{}, false
	}
	if enabled, _ := m["clientExternalConfig"].(bool); !enabled {
		return OutboundOption{}, false
	}
	tag, _ := m["tag"].(string)
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return OutboundOption{}, false
	}
	protocol, _ := m["protocol"].(string)
	remark, _ := m["remark"].(string)
	if remark == "" {
		remark = tag
	}
	return OutboundOption{Tag: tag, Protocol: protocol, Remark: remark, Source: source}, true
}

func (s *ClientService) OutboundOptions(xraySvc *XraySettingService, outboundSubSvc *OutboundSubscriptionService) ([]OutboundOption, error) {
	raw, err := s.OutboundOptionsWithRaw(xraySvc, outboundSubSvc)
	if err != nil {
		return nil, err
	}
	out := make([]OutboundOption, 0, len(raw))
	for _, item := range raw {
		if opt, ok := outboundOptionFromAny(item, ""); ok {
			source, _ := item["_source"].(string)
			opt.Source = source
			out = append(out, opt)
		}
	}
	return out, nil
}

func (s *ClientService) OutboundOptionsWithRaw(xraySvc *XraySettingService, outboundSubSvc *OutboundSubscriptionService) ([]map[string]any, error) {
	seen := map[string]struct{}{}
	out := []map[string]any{}
	add := func(raw any, source string) {
		opt, ok := outboundOptionFromAny(raw, source)
		if !ok {
			return
		}
		if _, dup := seen[opt.Tag]; dup {
			return
		}
		seen[opt.Tag] = struct{}{}
		if m, ok := raw.(map[string]any); ok {
			copied := map[string]any{}
			for k, v := range m {
				copied[k] = v
			}
			copied["_source"] = source
			out = append(out, copied)
		}
	}

	if xraySvc != nil {
		tpl, err := xraySvc.GetXrayConfigTemplate()
		if err != nil {
			return nil, err
		}
		var cfg map[string]any
		if err := json.Unmarshal([]byte(UnwrapXrayTemplateConfig(tpl)), &cfg); err != nil {
			return nil, err
		}
		if arr, ok := cfg["outbounds"].([]any); ok {
			for _, raw := range arr {
				add(raw, "template")
			}
		}
	}

	if outboundSubSvc != nil {
		arr, err := outboundSubSvc.AllActiveOutbounds()
		if err != nil {
			return nil, err
		}
		for _, raw := range arr {
			add(raw, "subscription")
		}
	}
	return out, nil
}

func (s *ClientService) AttachOutboundsByEmail(email string, tags []string) error {
	if strings.TrimSpace(email) == "" {
		return common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return err
	}
	current, err := s.GetOutboundTagsForRecord(rec.Id)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(current)+len(tags))
	for _, tag := range current {
		seen[tag] = struct{}{}
	}
	next := append([]string{}, current...)
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		next = append(next, tag)
	}
	return s.SetOutboundTagsForRecord(rec.Id, next)
}

func (s *ClientService) DetachOutboundsByEmail(email string, tags []string) error {
	if strings.TrimSpace(email) == "" {
		return common.NewError("client email is required")
	}
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return err
	}
	remove := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			remove[tag] = struct{}{}
		}
	}
	current, err := s.GetOutboundTagsForRecord(rec.Id)
	if err != nil {
		return err
	}
	next := make([]string, 0, len(current))
	for _, tag := range current {
		if _, ok := remove[tag]; !ok {
			next = append(next, tag)
		}
	}
	return s.SetOutboundTagsForRecord(rec.Id, next)
}
