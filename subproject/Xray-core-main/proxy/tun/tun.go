package tun

// Tun interface implements tun interface interaction
type Tun interface {
	Start() error
	Close() error
}

// TunOptions for tun interface implementation
type TunOptions struct {
	Name string
	MTU  uint32
}
