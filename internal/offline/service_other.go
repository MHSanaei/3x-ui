//go:build !linux

package offline

import (
	"errors"
	"io"
)

var errSystemd = errors.New("offline replacement requires a Linux systemd installation")

func StartUpload(io.Reader) (string, error) { return "", errSystemd }
func ApplyUpload(string, string) error      { return errSystemd }
func Apply(string, string) error            { return errSystemd }

func Install(string, string, string) error { return errSystemd }
