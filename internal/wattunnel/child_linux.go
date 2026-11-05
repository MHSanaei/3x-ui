//go:build linux

package wattunnel

import (
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

func canRawSocket() bool {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.IPPROTO_ICMP)
	if err != nil {
		return false
	}
	_ = unix.Close(fd)
	return true
}

func prepareChild(cmd *exec.Cmd, c Config, selected, dir string) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	if os.Geteuid() != 0 {
		return nil
	}
	account, err := user.Lookup("x-ui-wattunnel")
	if err != nil {
		return errors.New("create the x-ui-wattunnel service account before starting WatTunnel as root")
	}
	uid, err := strconv.ParseUint(account.Uid, 10, 32)
	if err != nil {
		return err
	}
	if uid == 0 {
		return errors.New("x-ui-wattunnel must be an unprivileged service account")
	}
	gid, err := strconv.ParseUint(account.Gid, 10, 32)
	if err != nil {
		return err
	}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), NoSetGroups: false}
	raw := selected == "icmp"
	if c.Carrier == "auto" && c.Role == "server" {
		for _, p := range c.Candidates {
			raw = raw || p.Carrier == "icmp"
		}
	}
	if raw {
		cmd.SysProcAttr.AmbientCaps = append(cmd.SysProcAttr.AmbientCaps, unix.CAP_NET_RAW)
	}
	for _, b := range c.Bindings() {
		if b.Port < 1024 {
			cmd.SysProcAttr.AmbientCaps = append(cmd.SysProcAttr.AmbientCaps, unix.CAP_NET_BIND_SERVICE)
			break
		}
	}
	if err = os.Chmod(filepath.Dir(dir), 0o711); err != nil {
		return err
	}
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("symlinks are forbidden in tunnel runtime directories")
		}
		return os.Chown(path, int(uid), int(gid))
	})
}

func openPreferredCarrier(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
