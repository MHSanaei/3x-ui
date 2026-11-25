package wechat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

var ErrReport = errors.New("wechat status report is unavailable")

type ServerProvider interface {
	GetStatus(*service.Status) *service.Status
}

type InboundProvider interface {
	GetAllInbounds() ([]*model.Inbound, error)
}

type reportCounts struct{ total, disabled, expired, depleted int }

type reportTraffic struct {
	used, remaining int64
	quotaUnset      bool
}

func addTrafficBytes(current, value int64) int64 {
	value = max(value, 0)
	const limit = int64(1<<63 - 1)
	return current + min(value, limit-current)
}

func (traffic *reportTraffic) add(up, down, total int64) {
	used := addTrafficBytes(max(up, 0), down)
	traffic.used = addTrafficBytes(traffic.used, used)
	if total <= 0 {
		traffic.quotaUnset = true
		return
	}
	traffic.remaining = addTrafficBytes(traffic.remaining, max(total-used, 0))
}

func (count *reportCounts) add(enabled bool, expiry, quota, up, down, now int64) {
	count.total++
	if !enabled {
		count.disabled++
	}
	if expiry > 0 && expiry <= now {
		count.expired++
	}
	if quota > 0 && (up >= quota || down >= quota || up >= quota-down) {
		count.depleted++
	}
}

func (s *Service) SendReport(ctx context.Context, server ServerProvider, inbound InboundProvider) error {
	if ctx == nil {
		ctx = context.Background()
	}
	on, err := s.settings.GetWechatEnable()
	if err != nil {
		return ErrConfiguration
	}
	if !on {
		return nil
	}
	enabled, err := s.settings.GetWechatReportEnable()
	if err != nil {
		return ErrConfiguration
	}
	if !enabled {
		return nil
	}
	if server == nil || inbound == nil {
		return ErrReport
	}
	status := server.GetStatus(nil)
	if status == nil {
		return ErrReport
	}
	inbounds, err := inbound.GetAllInbounds()
	if err != nil {
		return ErrReport
	}
	limits, err := reportClientLimits(ctx, inbounds)
	if err != nil {
		return ErrReport
	}
	tr := s.translator()
	now := time.Now()
	cpu := "—"
	if status.MetricAvailable("cpu") {
		cpu = fmt.Sprintf("%.2f%%", status.Cpu)
	}
	uptime := "—"
	if status.MetricAvailable("uptime") {
		uptime = tr("discord.values.uptime", fmt.Sprintf("Days==%d", status.Uptime/86400), fmt.Sprintf("Hours==%d", status.Uptime%86400/3600))
	}
	capacity := func(used, total uint64) string {
		if total == 0 {
			return "—"
		}
		return fmt.Sprintf("%s / %s", common.FormatTraffic(int64(used)), common.FormatTraffic(int64(total)))
	}
	inboundCounts, clientCounts := reportCounts{}, reportCounts{}
	traffic := reportTraffic{}
	seen := make(map[string]bool)
	for _, item := range inbounds {
		if item == nil {
			continue
		}
		inboundCounts.add(item.Enable, item.ExpiryTime, item.Total, item.Up, item.Down, now.UnixMilli())
		for _, client := range item.ClientStats {
			key := strings.ToLower(client.Email)
			if seen[key] {
				continue
			}
			seen[key] = true
			if limit, ok := limits[key]; ok {
				if client.Total == 0 {
					client.Total = limit.TotalGB
				}
				if client.ExpiryTime == 0 {
					client.ExpiryTime = limit.ExpiryTime
				}
			}
			clientCounts.add(client.Enable, client.ExpiryTime, client.Total, client.Up, client.Down, now.UnixMilli())
			traffic.add(client.Up, client.Down, client.Total)
		}
	}
	if clientCounts.total == 0 {
		traffic.quotaUnset = true
	}
	counts := func(value reportCounts) string {
		return tr("wechat.report.counts", fmt.Sprintf("Total==%d", value.total), fmt.Sprintf("Disabled==%d", value.disabled), fmt.Sprintf("Expired==%d", value.expired), fmt.Sprintf("Depleted==%d", value.depleted))
	}
	remaining := common.FormatTraffic(traffic.remaining)
	if traffic.quotaUnset {
		remaining = tr("wechat.report.trafficQuotaUnset")
	}
	text := strings.Join([]string{
		tr("wechat.report.title"),
		tr("discord.fields.panelVersion") + ": " + config.GetPanelVersion(),
		tr("discord.fields.xrayCore") + ": " + plain(status.Xray.Version) + " (" + string(status.Xray.State) + ")",
		"CPU: " + cpu,
		tr("pages.index.memory") + ": " + capacity(status.Mem.Current, status.Mem.Total),
		tr("pages.index.storage") + ": " + capacity(status.Disk.Current, status.Disk.Total),
		tr("pages.index.uptime") + ": " + uptime,
		tr("wechat.report.trafficUsed") + ": " + common.FormatTraffic(traffic.used),
		tr("wechat.report.trafficRemaining") + ": " + remaining,
		tr("tgbot.inbounds") + ": " + counts(inboundCounts),
		tr("clients") + ": " + counts(clientCounts),
	}, "\n")
	return s.send(ctx, "report", "report", text, now)
}
