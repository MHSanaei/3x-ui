package offline

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

type replacement struct {
	path, dir              string
	existed, changed, keep bool
}

func backup(path string) (replacement, error) {
	item := replacement{path: path, changed: true}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return item, nil
	}
	if err != nil {
		return item, err
	}
	if !info.Mode().IsRegular() {
		return item, fmt.Errorf("replacement destination must be a regular file: %s", path)
	}
	item.dir, err = os.MkdirTemp(filepath.Dir(path), ".x-ui-rollback-")
	if err != nil {
		return item, err
	}
	if err := copyFile(path, filepath.Join(item.dir, "previous"), info.Mode().Perm()); err != nil {
		_ = os.RemoveAll(item.dir)
		return item, err
	}
	item.existed = true
	return item, nil
}

func prepareReplacement(path, src string, mode os.FileMode) (replacement, error) {
	item, err := backup(path)
	if err != nil {
		return item, err
	}
	item.changed = false
	if item.dir == "" {
		item.dir, err = os.MkdirTemp(filepath.Dir(path), ".x-ui-rollback-")
		if err != nil {
			return item, err
		}
	}
	if err := copyFile(src, filepath.Join(item.dir, "next"), mode); err != nil {
		_ = os.RemoveAll(item.dir)
		return item, err
	}
	return item, nil
}

func (item *replacement) install() error {
	if err := os.Rename(filepath.Join(item.dir, "next"), item.path); err != nil {
		return err
	}
	item.changed = true
	return nil
}

func (item *replacement) restore() error {
	if !item.changed {
		return nil
	}
	if item.existed {
		if err := os.Rename(filepath.Join(item.dir, "previous"), item.path); err != nil {
			return fmt.Errorf("restore %s from %s: %w", item.path, item.dir, err)
		}
		return nil
	}
	if err := os.Remove(item.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func copyFile(source, dest string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return errors.Join(err, out.Chmod(mode), out.Sync(), out.Close())
}

func writeFile(path string, data []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".x-ui-write-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	if err := errors.Join(err, file.Chmod(mode), file.Sync(), file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func readyPath() string {
	return filepath.Join(config.GetDBFolderPath(), "offline-ready.pid")
}

func MarkReady(coreReady bool) error {
	status := strconv.Itoa(os.Getpid()) + " " + strconv.FormatBool(coreReady) + "\n"
	return writeFile(readyPath(), []byte(status), 0o600)
}
