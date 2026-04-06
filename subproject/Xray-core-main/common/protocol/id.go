package protocol

import (
	"crypto/md5"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/uuid"
)

const (
	IDBytesLen = 16
)

// The ID of en entity, in the form of a UUID.
type ID struct {
	uuid   uuid.UUID
	cmdKey [IDBytesLen]byte
}

// Equals returns true if this ID equals to the other one.
func (id *ID) Equals(another *ID) bool {
	return id.uuid.Equals(&(another.uuid))
}

func (id *ID) Bytes() []byte {
	return id.uuid.Bytes()
}

func (id *ID) String() string {
	return id.uuid.String()
}

func (id *ID) UUID() uuid.UUID {
	return id.uuid
}

func (id ID) CmdKey() []byte {
	return id.cmdKey[:]
}

// NewID returns an ID with given UUID.
func NewID(uuid uuid.UUID) *ID {
	id := &ID{uuid: uuid}
	md5hash := md5.New()
	common.Must2(md5hash.Write(uuid.Bytes()))
	common.Must2(md5hash.Write([]byte("c48619fe-8f02-49e0-b9e9-edf763e17e21")))
	md5hash.Sum(id.cmdKey[:0])
	return id
}
