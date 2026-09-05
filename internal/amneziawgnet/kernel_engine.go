package amneziawgnet

import (
	"fmt"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// KernelEngine manages native Linux kernel AmneziaWG interfaces.
type KernelEngine struct {
	mu     sync.Mutex
	ifaces map[int]amneziawg.Instance
}

// NewKernelEngine returns a new native kernel AmneziaWG engine.
func NewKernelEngine() *KernelEngine {
	return &KernelEngine{
		ifaces: make(map[int]amneziawg.Instance),
	}
}

func (k *KernelEngine) Name() string {
	return "kernel"
}

func (k *KernelEngine) HasRunning() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.ifaces) > 0
}

func (k *KernelEngine) Ensure(d Desired) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	inst := d.Instance
	ifaceName := inst.InterfaceName
	if ifaceName == "" {
		ifaceName = fmt.Sprintf("awg%d", inst.Id)
	}

	mtu := inst.MTU
	if mtu <= 0 {
		mtu = defaultMTU
	}

	if err := createKernelLink(ifaceName, mtu, inst.Address); err != nil {
		return fmt.Errorf("kernel link: %w", err)
	}

	uapiConf, err := buildUAPIConfig(inst, d.Options)
	if err != nil {
		return fmt.Errorf("build UAPI config: %w", err)
	}

	if err := applyKernelUAPI(ifaceName, uapiConf); err != nil {
		return fmt.Errorf("apply kernel UAPI: %w", err)
	}

	if err := setupKernelRouting(ifaceName, inst.Id, inst.Address); err != nil {
		logger.Warningf("amneziawgnet: kernel routing setup for %s: %v", ifaceName, err)
	}

	k.ifaces[inst.Id] = inst
	logger.Infof("amneziawgnet: started kernel interface %s for inbound %d", ifaceName, inst.Id)
	return nil
}

func (k *KernelEngine) Remove(inboundID int) {
	k.mu.Lock()
	defer k.mu.Unlock()

	inst, exists := k.ifaces[inboundID]
	if !exists {
		return
	}

	ifaceName := inst.InterfaceName
	if ifaceName == "" {
		ifaceName = fmt.Sprintf("awg%d", inst.Id)
	}

	_ = teardownKernelRouting(ifaceName, inboundID)
	delete(k.ifaces, inboundID)
	logger.Infof("amneziawgnet: stopped kernel interface %s for inbound %d", ifaceName, inboundID)
}

func (k *KernelEngine) StopAll() {
	k.mu.Lock()
	defer k.mu.Unlock()

	for id, inst := range k.ifaces {
		ifaceName := inst.InterfaceName
		if ifaceName == "" {
			ifaceName = fmt.Sprintf("awg%d", inst.Id)
		}
		_ = teardownKernelRouting(ifaceName, id)
		delete(k.ifaces, id)
	}
}

func (k *KernelEngine) Diagnose(inboundID int, peers []amneziawg.Peer) Diagnostics {
	k.mu.Lock()
	defer k.mu.Unlock()

	inst, exists := k.ifaces[inboundID]
	if !exists {
		return Diagnostics{}
	}

	ifaceName := inst.InterfaceName
	if ifaceName == "" {
		ifaceName = fmt.Sprintf("awg%d", inst.Id)
	}

	dump, err := readKernelUAPIDump(ifaceName)
	if err != nil {
		return Diagnostics{
			Running:    true,
			Engine:     "kernel",
			ListenPort: inst.ListenPort,
		}
	}

	listenPort, _ := parseUAPIDump(dump)
	if listenPort == 0 {
		listenPort = inst.ListenPort
	}

	return Diagnostics{
		Running:    true,
		Engine:     "kernel",
		ListenPort: listenPort,
	}
}
