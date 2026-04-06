//go:build linux && !android

package tun

import (
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// LinuxTun is an object that handles tun network interface on linux
// current version is heavily stripped to do nothing more,
// then create a network interface, to be provided as file descriptor to gVisor ip stack
type LinuxTun struct {
	tunFd   int
	tunLink netlink.Link
	options TunOptions
}

// LinuxTun implements Tun
var _ Tun = (*LinuxTun)(nil)

// LinuxTun implements GVisorTun
var _ GVisorTun = (*LinuxTun)(nil)

// NewTun builds new tun interface handler (linux specific)
func NewTun(options TunOptions) (Tun, error) {
	tunFd, err := open(options.Name)
	if err != nil {
		return nil, err
	}

	tunLink, err := setup(options.Name, int(options.MTU))
	if err != nil {
		_ = unix.Close(tunFd)
		return nil, err
	}

	linuxTun := &LinuxTun{
		tunFd:   tunFd,
		tunLink: tunLink,
		options: options,
	}

	return linuxTun, nil
}

// open the file that implements tun interface in the OS
func open(name string) (int, error) {
	fd, err := unix.Open("/dev/net/tun", unix.O_RDWR, 0)
	if err != nil {
		return -1, err
	}

	ifr, err := unix.NewIfreq(name)
	if err != nil {
		_ = unix.Close(fd)
		return 0, err
	}

	flags := unix.IFF_TUN | unix.IFF_NO_PI
	ifr.SetUint16(uint16(flags))
	err = unix.IoctlIfreq(fd, unix.TUNSETIFF, ifr)
	if err != nil {
		_ = unix.Close(fd)
		return 0, err
	}

	err = unix.SetNonblock(fd, true)
	if err != nil {
		_ = unix.Close(fd)
		return 0, err
	}

	return fd, nil
}

// setup the interface through netlink socket
func setup(name string, MTU int) (netlink.Link, error) {
	tunLink, err := netlink.LinkByName(name)
	if err != nil {
		return nil, err
	}

	err = netlink.LinkSetMTU(tunLink, MTU)
	if err != nil {
		_ = netlink.LinkSetDown(tunLink)
		return nil, err
	}

	return tunLink, nil
}

// Start is called by handler to bring tun interface to life
func (t *LinuxTun) Start() error {
	err := netlink.LinkSetUp(t.tunLink)
	if err != nil {
		return err
	}

	return nil
}

// Close is called to shut down the tun interface
func (t *LinuxTun) Close() error {
	_ = netlink.LinkSetDown(t.tunLink)
	_ = unix.Close(t.tunFd)

	return nil
}

// newEndpoint builds new gVisor stack.LinkEndpoint from the tun interface file descriptor
func (t *LinuxTun) newEndpoint() (stack.LinkEndpoint, error) {
	return fdbased.New(&fdbased.Options{
		FDs:               []int{t.tunFd},
		MTU:               t.options.MTU,
		RXChecksumOffload: true,
	})
}
