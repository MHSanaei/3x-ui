package service

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type ClientPendingOperation struct {
	Token string `json:"token"`
	Owner int64  `json:"owner"`
}

func pendingClientOperationKey(recordID int) string {
	return "tgbot.clientPending.v1." + strconv.Itoa(recordID)
}

func clientResetOperationKey(recordID int) string {
	return "tgbot.clientReset.v1." + strconv.Itoa(recordID)
}

func pendingClientOperations(tx *gorm.DB, recordID int) ([]ClientPendingOperation, error) {
	var pending []ClientPendingOperation
	err := readBotSetting(tx, pendingClientOperationKey(recordID), &pending)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return pending, err
	}
	// Lazily index receipts written before the per-client pending index existed.
	var rows []model.Setting
	if err := tx.Select("key", "value").Where("key >= ? AND key < ? AND value LIKE ? AND value LIKE ?", clientOperationSettingPrefix, clientOperationSettingPrefix+"~", `%"state":"committed"%`, `%"recordId":`+strconv.Itoa(recordID)+`,%`).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		var op ClientAdminOperation
		if err := json.Unmarshal([]byte(row.Value), &op); err != nil {
			return nil, err
		}
		if op.RecordID == recordID && op.State == "committed" {
			pending = append(pending, ClientPendingOperation{op.Token, op.Owner})
		}
	}
	return pending, writeBotSetting(tx, pendingClientOperationKey(recordID), pending)
}

func registerPendingClientOperation(tx *gorm.DB, op *ClientAdminOperation) error {
	pending, err := pendingClientOperations(tx, op.RecordID)
	if err != nil {
		return err
	}
	entry := ClientPendingOperation{op.Token, op.Owner}
	if !slices.Contains(pending, entry) {
		pending = append(pending, entry)
	}
	return writeBotSetting(tx, pendingClientOperationKey(op.RecordID), pending)
}

func unregisterPendingClientOperation(tx *gorm.DB, op *ClientAdminOperation) error {
	pending, err := pendingClientOperations(tx, op.RecordID)
	if err != nil {
		return err
	}
	pending = slices.DeleteFunc(pending, func(entry ClientPendingOperation) bool { return entry.Token == op.Token })
	return writeBotSetting(tx, pendingClientOperationKey(op.RecordID), pending)
}
