//go:build windows

package tun

import (
	"crypto/md5"
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

//go:linkname procyield runtime.procyield
func procyield(cycles uint32)

// WindowsTun is an object that handles tun network interface on Windows
// current version is heavily stripped to do nothing more,
// then create a network interface, to be provided as endpoint to gVisor ip stack
type WindowsTun struct {
	options  TunOptions
	adapter  *wintun.Adapter
	session  wintun.Session
	readWait windows.Handle
	MTU      uint32
}

// WindowsTun implements Tun
var _ Tun = (*WindowsTun)(nil)

// WindowsTun implements GVisorTun
var _ GVisorTun = (*WindowsTun)(nil)

// WindowsTun implements GVisorDevice
var _ GVisorDevice = (*WindowsTun)(nil)

// NewTun creates a Wintun interface with the given name. Should a Wintun
// interface with the same name exist, it tried to be reused.
func NewTun(options TunOptions) (Tun, error) {
	// instantiate wintun adapter
	adapter, err := open(options.Name)
	if err != nil {
		return nil, err
	}

	// start the interface with ring buffer capacity of 8 MiB
	session, err := adapter.StartSession(0x800000)
	if err != nil {
		_ = adapter.Close()
		return nil, err
	}

	tun := &WindowsTun{
		options:  options,
		adapter:  adapter,
		session:  session,
		readWait: session.ReadWaitEvent(),
		// there is currently no iphndl.dll support, which is the netlink library for windows
		// so there is nowhere to change MTU for the Wintun interface, and we take its default value
		MTU: wintun.PacketSizeMax,
	}

	return tun, nil
}

func open(name string) (*wintun.Adapter, error) {
	// generate a deterministic GUID from the adapter name
	id := md5.Sum([]byte(name))
	guid := (*windows.GUID)(unsafe.Pointer(&id[0]))
	// try to open existing adapter by name
	adapter, err := wintun.OpenAdapter(name)
	if err == nil {
		return adapter, nil
	}
	// try to create adapter anew
	adapter, err = wintun.CreateAdapter(name, "Xray", guid)
	if err == nil {
		return adapter, nil
	}
	return nil, err
}

func (t *WindowsTun) Start() error {
	return nil
}

func (t *WindowsTun) Close() error {
	t.session.End()
	_ = t.adapter.Close()

	return nil
}

// WritePacket implements GVisorDevice method to write one packet to the tun device
func (t *WindowsTun) WritePacket(packetBuffer *stack.PacketBuffer) tcpip.Error {
	// request buffer from Wintun
	packet, err := t.session.AllocateSendPacket(packetBuffer.Size())
	if err != nil {
		return &tcpip.ErrAborted{}
	}

	// copy the bytes of slices that compose the packet into the allocated buffer
	var index int
	for _, packetElement := range packetBuffer.AsSlices() {
		index += copy(packet[index:], packetElement)
	}

	// signal Wintun to send that buffer as the packet
	t.session.SendPacket(packet)

	return nil
}

// ReadPacket implements GVisorDevice method to read one packet from the tun device
// It is expected that the method will not block, rather return ErrQueueEmpty when there is nothing on the line,
// which will make the stack call Wait which should implement desired push-back
func (t *WindowsTun) ReadPacket() (byte, *stack.PacketBuffer, error) {
	packet, err := t.session.ReceivePacket()
	if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
		return 0, nil, ErrQueueEmpty
	}
	if err != nil {
		return 0, nil, err
	}

	version := packet[0] >> 4
	packetBuffer := buffer.MakeWithView(buffer.NewViewWithData(packet))
	return version, stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload:           packetBuffer,
		IsForwardedPacket: true,
		OnRelease: func() {
			t.session.ReleaseReceivePacket(packet)
		},
	}), nil
}

func (t *WindowsTun) Wait() {
	procyield(1)
	_, _ = windows.WaitForSingleObject(t.readWait, windows.INFINITE)
}

func (t *WindowsTun) newEndpoint() (stack.LinkEndpoint, error) {
	return &LinkEndpoint{deviceMTU: t.options.MTU, device: t}, nil
}
