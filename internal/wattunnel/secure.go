package wattunnel

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/crypto/nodetoken"
)

var keyMu sync.Mutex

func rejectSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("symlinks forbidden in private tunnel storage")
	}
	return nil
}

func ProtectDir(dir string) error {
	if err := rejectSymlink(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return protect(dir, true)
}

func WritePrivate(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := ProtectDir(dir); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".wattun-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = protect(tmp, false); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func SecretCodec(dir string, create bool) (*nodetoken.Codec, error) {
	keyMu.Lock()
	defer keyMu.Unlock()
	path := filepath.Join(dir, "keyring.json")
	if err := rejectSymlink(path); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) && create {
		key, e := NewKey()
		if e != nil {
			return nil, e
		}
		// A separate keyring keeps enabling Tunnels independent of node-token policy.
		raw, e := json.Marshal(map[string]any{"active": "v1", "keys": map[string]string{"v1": hexKeyBase64(key)}})
		if e != nil {
			return nil, e
		}
		if e = WritePrivate(path, raw); e != nil {
			return nil, e
		}
	}
	if err := protect(path, false); err != nil {
		return nil, err
	}
	ring, err := (nodetoken.FileKeySource{Path: path}).Load()
	if err != nil {
		return nil, err
	}
	return nodetoken.NewCodec(nodetoken.ModeRequired, ring)
}
