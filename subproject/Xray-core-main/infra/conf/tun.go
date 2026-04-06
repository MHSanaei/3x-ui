package conf

import (
	"github.com/xtls/xray-core/proxy/tun"
	"google.golang.org/protobuf/proto"
)

type TunConfig struct {
	Name      string `json:"name"`
	MTU       uint32 `json:"MTU"`
	UserLevel uint32 `json:"userLevel"`
}

func (v *TunConfig) Build() (proto.Message, error) {
	config := &tun.Config{
		Name:      v.Name,
		MTU:       v.MTU,
		UserLevel: v.UserLevel,
	}

	if v.Name == "" {
		config.Name = "xray0"
	}

	if v.MTU == 0 {
		config.MTU = 1500
	}

	return config, nil
}
