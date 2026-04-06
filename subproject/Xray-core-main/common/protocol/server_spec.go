package protocol

import (
	"github.com/xtls/xray-core/common/net"
)

type ServerSpec struct {
	Destination  net.Destination
	User         *MemoryUser
}

func NewServerSpec(dest net.Destination, user *MemoryUser) *ServerSpec {
	return &ServerSpec{
		Destination: dest,
		User:        user,
	}
}

func NewServerSpecFromPB(spec *ServerEndpoint) (*ServerSpec, error) {
	dest := net.TCPDestination(spec.Address.AsAddress(), net.Port(spec.Port))
	var dUser *MemoryUser
	if spec.User != nil {
		user, err := spec.User.ToMemoryUser()
		if err != nil {
			return nil, err
		}
		dUser = user
	}
	return NewServerSpec(dest, dUser), nil
}
