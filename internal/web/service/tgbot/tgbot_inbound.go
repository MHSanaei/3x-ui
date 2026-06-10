package tgbot

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// getInboundUsages retrieves and formats inbound usage information.
func (t *Tgbot) getInboundUsages() string {
	var info strings.Builder
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("GetAllInbounds run failed:", err)
		info.WriteString(t.I18nBot("tgbot.answers.getInboundsFailed"))
		return info.String()
	}
	for _, inbound := range inbounds {
		info.WriteString(t.I18nBot("tgbot.messages.inbound", "Remark=="+inbound.Remark))
		info.WriteString(t.I18nBot("tgbot.messages.port", "Port=="+strconv.Itoa(inbound.Port)))
		info.WriteString(t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic((inbound.Up+inbound.Down)), "Upload=="+common.FormatTraffic(inbound.Up), "Download=="+common.FormatTraffic(inbound.Down)))

		clients, listErr := t.clientService.ListForInbound(nil, inbound.Id)
		if listErr == nil {
			info.WriteString(fmt.Sprintf("👥 Clients: %d\r\n", len(clients)))
		}

		if inbound.ExpiryTime == 0 {
			info.WriteString(t.I18nBot("tgbot.messages.expire", "Time=="+t.I18nBot("tgbot.unlimited")))
		} else {
			info.WriteString(t.I18nBot("tgbot.messages.expire", "Time=="+time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05")))
		}
		info.WriteString("\r\n")
	}
	return info.String()
}

// getInbounds creates an inline keyboard with all inbounds.
func (t *Tgbot) getInbounds() (*telego.InlineKeyboardMarkup, error) {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("GetAllInbounds run failed:", err)
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}

	if len(inbounds) == 0 {
		logger.Warning("No inbounds found")
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}

	var buttons []telego.InlineKeyboardButton
	for _, inbound := range inbounds {
		status := "❌"
		if inbound.Enable {
			status = "✅"
		}
		callbackData := t.encodeQuery(fmt.Sprintf("%s %d", "get_clients", inbound.Id))
		buttons = append(buttons, tu.InlineKeyboardButton(fmt.Sprintf("%v - %v", inbound.Remark, status)).WithCallbackData(callbackData))
	}

	cols := 1
	if len(buttons) >= 6 {
		cols = 2
	}

	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
	return keyboard, nil
}

// getInboundsFor builds an inline keyboard of inbounds for a custom next action.
func (t *Tgbot) getInboundsFor(nextAction string) (*telego.InlineKeyboardMarkup, error) {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("GetAllInbounds run failed:", err)
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}

	if len(inbounds) == 0 {
		logger.Warning("No inbounds found")
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}

	var buttons []telego.InlineKeyboardButton
	for _, inbound := range inbounds {
		status := "❌"
		if inbound.Enable {
			status = "✅"
		}
		callbackData := t.encodeQuery(fmt.Sprintf("%s %d", nextAction, inbound.Id))
		buttons = append(buttons, tu.InlineKeyboardButton(fmt.Sprintf("%v - %v", inbound.Remark, status)).WithCallbackData(callbackData))
	}

	cols := 1
	if len(buttons) >= 6 {
		cols = 2
	}

	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
	return keyboard, nil
}

// getInboundClientsFor lists clients of an inbound with a specific action prefix to be appended with email
func (t *Tgbot) getInboundClientsFor(inboundID int, action string) (*telego.InlineKeyboardMarkup, error) {
	inbound, err := t.inboundService.GetInbound(inboundID)
	if err != nil {
		logger.Warning("getInboundClientsFor run failed:", err)
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}
	clients, err := t.inboundService.GetClients(inbound)
	var buttons []telego.InlineKeyboardButton

	if err != nil {
		logger.Warning("GetInboundClients run failed:", err)
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	} else {
		if len(clients) > 0 {
			for _, client := range clients {
				buttons = append(buttons, tu.InlineKeyboardButton(client.Email).WithCallbackData(t.encodeQuery(action+" "+client.Email)))
			}

		} else {
			return nil, errors.New(t.I18nBot("tgbot.answers.getClientsFailed"))
		}

	}
	cols := 0
	if len(buttons) < 6 {
		cols = 3
	} else {
		cols = 2
	}
	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))

	return keyboard, nil
}

// getInboundsAddClient creates an inline keyboard for adding clients to inbounds.
func (t *Tgbot) getInboundsAddClient() (*telego.InlineKeyboardMarkup, error) {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("GetAllInbounds run failed:", err)
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}

	if len(inbounds) == 0 {
		logger.Warning("No inbounds found")
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}

	excludedProtocols := map[model.Protocol]bool{
		model.Tunnel:    true,
		model.Mixed:     true,
		model.WireGuard: true,
		model.HTTP:      true,
	}

	var buttons []telego.InlineKeyboardButton
	for _, inbound := range inbounds {
		if excludedProtocols[inbound.Protocol] {
			continue
		}

		status := "❌"
		if inbound.Enable {
			status = "✅"
		}
		callbackData := t.encodeQuery(fmt.Sprintf("%s %d", "add_client_to", inbound.Id))
		buttons = append(buttons, tu.InlineKeyboardButton(fmt.Sprintf("%v - %v", inbound.Remark, status)).WithCallbackData(callbackData))
	}

	cols := 1
	if len(buttons) >= 6 {
		cols = 2
	}

	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
	return keyboard, nil
}

// getInboundsAttachPicker builds a toggle picker over multi-client inbounds
// for the "attach more inbounds to the new client" step. Each row shows the
// current selection state for the inbound; tapping fires
// add_client_toggle_attach <id> which flips it and re-renders. A final
// "Done" button (add_client_attach_done) returns to the field-edit screen.
func (t *Tgbot) getInboundsAttachPicker() (*telego.InlineKeyboardMarkup, error) {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("GetAllInbounds run failed:", err)
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}
	if len(inbounds) == 0 {
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}
	excludedProtocols := map[model.Protocol]bool{
		model.Tunnel:    true,
		model.Mixed:     true,
		model.WireGuard: true,
		model.HTTP:      true,
	}
	selected := make(map[int]bool, len(receiver_inbound_IDs))
	for _, id := range receiver_inbound_IDs {
		selected[id] = true
	}
	var buttons []telego.InlineKeyboardButton
	for _, ib := range inbounds {
		if excludedProtocols[ib.Protocol] {
			continue
		}
		mark := "☐"
		if selected[ib.Id] {
			mark = "✅"
		}
		label := fmt.Sprintf("%s %s (%s)", mark, ib.Remark, ib.Protocol)
		callback := t.encodeQuery(fmt.Sprintf("add_client_toggle_attach %d", ib.Id))
		buttons = append(buttons, tu.InlineKeyboardButton(label).WithCallbackData(callback))
	}
	cols := 1
	if len(buttons) >= 6 {
		cols = 2
	}
	rows := tu.InlineKeyboardCols(cols, buttons...)
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("✅ Done").WithCallbackData(t.encodeQuery("add_client_attach_done")),
	))
	return tu.InlineKeyboardGrid(rows), nil
}

// getInboundClients creates an inline keyboard with clients of a specific inbound.
func (t *Tgbot) getInboundClients(id int) (*telego.InlineKeyboardMarkup, error) {
	inbound, err := t.inboundService.GetInbound(id)
	if err != nil {
		logger.Warning("getIboundClients run failed:", err)
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}
	clients, err := t.inboundService.GetClients(inbound)
	var buttons []telego.InlineKeyboardButton

	if err != nil {
		logger.Warning("GetInboundClients run failed:", err)
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	} else {
		if len(clients) > 0 {
			for _, client := range clients {
				buttons = append(buttons, tu.InlineKeyboardButton(client.Email).WithCallbackData(t.encodeQuery("client_get_usage "+client.Email)))
			}

		} else {
			return nil, errors.New(t.I18nBot("tgbot.answers.getClientsFailed"))
		}

	}
	cols := 0
	if len(buttons) < 6 {
		cols = 3
	} else {
		cols = 2
	}
	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))

	return keyboard, nil
}

// searchInbound searches for inbounds by remark and sends the results.
func (t *Tgbot) searchInbound(chatId int64, remark string) {
	inbounds, err := t.inboundService.SearchInbounds(remark)
	if err != nil {
		logger.Warning(err)
		msg := t.I18nBot("tgbot.wentWrong")
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if len(inbounds) == 0 {
		msg := t.I18nBot("tgbot.noInbounds")
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	for _, inbound := range inbounds {
		info := ""
		info += t.I18nBot("tgbot.messages.inbound", "Remark=="+inbound.Remark)
		info += t.I18nBot("tgbot.messages.port", "Port=="+strconv.Itoa(inbound.Port))
		info += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic((inbound.Up+inbound.Down)), "Upload=="+common.FormatTraffic(inbound.Up), "Download=="+common.FormatTraffic(inbound.Down))

		if inbound.ExpiryTime == 0 {
			info += t.I18nBot("tgbot.messages.expire", "Time=="+t.I18nBot("tgbot.unlimited"))
		} else {
			info += t.I18nBot("tgbot.messages.expire", "Time=="+time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
		}
		t.SendMsgToTgbot(chatId, info)

		if len(inbound.ClientStats) > 0 {
			var output strings.Builder
			for _, traffic := range inbound.ClientStats {
				output.WriteString(t.clientInfoMsg(&traffic, true, true, true, true, true, true))
			}
			t.SendMsgToTgbot(chatId, output.String())
		}
	}
}
