package conf

import (
	"github.com/xtls/xray-core/app/metrics"
	"github.com/xtls/xray-core/common/errors"
)

type MetricsConfig struct {
	Tag    string `json:"tag"`
	Listen string `json:"listen"`
}

func (c *MetricsConfig) Build() (*metrics.Config, error) {
	if c.Listen == "" && c.Tag == "" {
		return nil, errors.New("Metrics must have a tag or listen address.")
	}
	// If the tag is empty but have "listen" set a default "Metrics" for compatibility.
	if c.Tag == "" {
		c.Tag = "Metrics"
	}

	return &metrics.Config{
		Tag:    c.Tag,
		Listen: c.Listen,
	}, nil
}
