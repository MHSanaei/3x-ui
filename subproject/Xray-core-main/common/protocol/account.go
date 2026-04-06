package protocol

import "google.golang.org/protobuf/proto"

// Account is a user identity used for authentication.
type Account interface {
	Equals(Account) bool
	ToProto() proto.Message
}

// AsAccount is an object can be converted into account.
type AsAccount interface {
	AsAccount() (Account, error)
}
