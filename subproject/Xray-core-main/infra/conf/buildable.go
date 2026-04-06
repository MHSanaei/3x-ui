package conf

import "google.golang.org/protobuf/proto"

type Buildable interface {
	Build() (proto.Message, error)
}
