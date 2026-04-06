package encoding

import (
	"encoding/binary"
	"io"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/protocol"
)

var (
	ErrCommandTooLarge     = errors.New("Command too large.")
	ErrCommandTypeMismatch = errors.New("Command type mismatch.")
	ErrInvalidAuth         = errors.New("Invalid auth.")
	ErrInsufficientLength  = errors.New("Insufficient length.")
	ErrUnknownCommand      = errors.New("Unknown command.")
)

func MarshalCommand(command interface{}, writer io.Writer) error {
	if command == nil {
		return ErrUnknownCommand
	}

	var cmdID byte
	var factory CommandFactory
	switch command.(type) {
	default:
		return ErrUnknownCommand
	}

	buffer := buf.New()
	defer buffer.Release()

	err := factory.Marshal(command, buffer)
	if err != nil {
		return err
	}

	auth := Authenticate(buffer.Bytes())
	length := buffer.Len() + 4
	if length > 255 {
		return ErrCommandTooLarge
	}

	common.Must2(writer.Write([]byte{cmdID, byte(length), byte(auth >> 24), byte(auth >> 16), byte(auth >> 8), byte(auth)}))
	common.Must2(writer.Write(buffer.Bytes()))
	return nil
}

func UnmarshalCommand(cmdID byte, data []byte) (protocol.ResponseCommand, error) {
	if len(data) <= 4 {
		return nil, ErrInsufficientLength
	}
	expectedAuth := Authenticate(data[4:])
	actualAuth := binary.BigEndian.Uint32(data[:4])
	if expectedAuth != actualAuth {
		return nil, ErrInvalidAuth
	}

	var factory CommandFactory
	switch cmdID {
	default:
		return nil, ErrUnknownCommand
	}
	return factory.Unmarshal(data[4:])
}

type CommandFactory interface {
	Marshal(command interface{}, writer io.Writer) error
	Unmarshal(data []byte) (interface{}, error)
}
