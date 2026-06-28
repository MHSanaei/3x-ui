//go:build !linux

package tc

import "github.com/mhsanaei/3x-ui/v3/internal/database/model"

func ApplyClientLimit(_ model.Client) error { return nil }

func RemoveClientLimitByEmail(_ string) error { return nil }
