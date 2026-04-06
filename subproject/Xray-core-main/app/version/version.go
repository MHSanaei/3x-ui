package version

import (
	"context"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"strconv"
	"strings"
)

type Version struct {
	config *Config
	ctx    context.Context
}

func New(ctx context.Context, config *Config) (*Version, error) {
	if config.MinVersion != "" {
		result, err := compareVersions(config.MinVersion, config.CoreVersion)
		if err != nil {
			return nil, err
		}
		if result > 0 {
			return nil, errors.New("this config must be run on version ", config.MinVersion, " or higher")
		}
	}
	if config.MaxVersion != "" {
		result, err := compareVersions(config.MaxVersion, config.CoreVersion)
		if err != nil {
			return nil, err
		}
		if result < 0 {
			return nil, errors.New("this config should be run on version ", config.MaxVersion, " or lower")
		}
	}
	return &Version{config: config, ctx: ctx}, nil
}

func compareVersions(v1, v2 string) (int, error) {
	// Split version strings into components
	v1Parts := strings.Split(v1, ".")
	v2Parts := strings.Split(v2, ".")

	// Pad shorter versions with zeros
	for len(v1Parts) < len(v2Parts) {
		v1Parts = append(v1Parts, "0")
	}
	for len(v2Parts) < len(v1Parts) {
		v2Parts = append(v2Parts, "0")
	}

	// Compare each part
	for i := 0; i < len(v1Parts); i++ {
		// Convert parts to integers
		n1, err := strconv.Atoi(v1Parts[i])
		if err != nil {
			return 0, errors.New("invalid version component ", v1Parts[i], " in ", v1)
		}
		n2, err := strconv.Atoi(v2Parts[i])
		if err != nil {
			return 0, errors.New("invalid version component ", v2Parts[i], " in ", v2)
		}

		if n1 < n2 {
			return -1, nil // v1 < v2
		}
		if n1 > n2 {
			return 1, nil // v1 > v2
		}
	}
	return 0, nil // v1 == v2
}

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, config interface{}) (interface{}, error) {
		return New(ctx, config.(*Config))
	}))
}
