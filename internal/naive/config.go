package naive

import (
	"fmt"
	"path/filepath"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type Config struct {
	Listen              string `json:"listen"`
	Proxy               string `json:"proxy"`
	InsecureConcurrency int    `json:"insecure-concurrency,omitempty"`
	TunnelTimeout       int    `json:"tunnel-timeout,omitempty"`
	IdleTimeout         int    `json:"idle-timeout,omitempty"`
	ExtraHeaders        string `json:"extra-headers,omitempty"`
	HostResolverRules   string `json:"host-resolver-rules,omitempty"`
	ResolverRange       string `json:"resolver-range,omitempty"`
	Log                 string `json:"log,omitempty"`
	NoPostQuantum       bool   `json:"no-post-quantum,omitempty"`
}

func ToConfig(outbound *model.NaiveOutbound) Config {
	return Config{
		Listen:              fmt.Sprintf("socks://127.0.0.1:%d", outbound.LocalPort),
		Proxy:               outbound.ProxyURL,
		InsecureConcurrency: outbound.InsecureConcurrency,
		TunnelTimeout:       outbound.TunnelTimeout,
		IdleTimeout:         outbound.IdleTimeout,
		ExtraHeaders:        outbound.ExtraHeaders,
		HostResolverRules:   outbound.HostResolverRules,
		ResolverRange:       outbound.ResolverRange,
		Log:                 filepath.Join(config.GetLogFolder(), fmt.Sprintf("naive-%s.log", outbound.Tag)),
		NoPostQuantum:       outbound.NoPostQuantum,
	}
}
