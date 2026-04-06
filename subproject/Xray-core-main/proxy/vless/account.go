package vless

import (
	"google.golang.org/protobuf/proto"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/uuid"
)

// AsAccount implements protocol.Account.AsAccount().
func (a *Account) AsAccount() (protocol.Account, error) {
	id, err := uuid.ParseString(a.Id)
	if err != nil {
		return nil, errors.New("failed to parse ID").Base(err).AtError()
	}
	return &MemoryAccount{
		ID:         protocol.NewID(id),
		Flow:       a.Flow,       // needs parser here?
		Encryption: a.Encryption, // needs parser here?
		XorMode:    a.XorMode,
		Seconds:    a.Seconds,
		Padding:    a.Padding,
		Reverse:    a.Reverse,
		Testpre:    a.Testpre,
		Testseed:   a.Testseed,
	}, nil
}

// MemoryAccount is an in-memory form of VLess account.
type MemoryAccount struct {
	// ID of the account.
	ID *protocol.ID
	// Flow of the account. May be "xtls-rprx-vision".
	Flow string

	Encryption string
	XorMode    uint32
	Seconds    uint32
	Padding    string

	Reverse *Reverse

	Testpre  uint32
	Testseed []uint32
}

// Equals implements protocol.Account.Equals().
func (a *MemoryAccount) Equals(account protocol.Account) bool {
	vlessAccount, ok := account.(*MemoryAccount)
	if !ok {
		return false
	}
	return a.ID.Equals(vlessAccount.ID)
}

func (a *MemoryAccount) ToProto() proto.Message {
	return &Account{
		Id:         a.ID.String(),
		Flow:       a.Flow,
		Encryption: a.Encryption,
		XorMode:    a.XorMode,
		Seconds:    a.Seconds,
		Padding:    a.Padding,
		Reverse:    a.Reverse,
		Testpre:    a.Testpre,
		Testseed:   a.Testseed,
	}
}
