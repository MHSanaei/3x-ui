package wireguard

import (
	"context"

	"github.com/xtls/xray-core/common/errors"
)

func (c *DeviceConfig) preferIP4() bool {
	return c.DomainStrategy == DeviceConfig_FORCE_IP ||
		c.DomainStrategy == DeviceConfig_FORCE_IP4 ||
		c.DomainStrategy == DeviceConfig_FORCE_IP46
}

func (c *DeviceConfig) preferIP6() bool {
	return c.DomainStrategy == DeviceConfig_FORCE_IP ||
		c.DomainStrategy == DeviceConfig_FORCE_IP6 ||
		c.DomainStrategy == DeviceConfig_FORCE_IP64
}

func (c *DeviceConfig) hasFallback() bool {
	return c.DomainStrategy == DeviceConfig_FORCE_IP46 || c.DomainStrategy == DeviceConfig_FORCE_IP64
}

func (c *DeviceConfig) fallbackIP4() bool {
	return c.DomainStrategy == DeviceConfig_FORCE_IP64
}

func (c *DeviceConfig) fallbackIP6() bool {
	return c.DomainStrategy == DeviceConfig_FORCE_IP46
}

func (c *DeviceConfig) createTun() tunCreator {
	if !c.IsClient {
		// See tun_linux.go createKernelTun()
		errors.LogWarning(context.Background(), "Using gVisor TUN. WG inbound doesn't support kernel TUN yet.")
		return createGVisorTun
	}
	if c.NoKernelTun {
		errors.LogWarning(context.Background(), "Using gVisor TUN. NoKernelTun is set to true.")
		return createGVisorTun
	}
	kernelTunSupported, err := KernelTunSupported()
	if err != nil {
		errors.LogWarning(context.Background(), "Using gVisor TUN. Failed to check kernel TUN support:", err)
		return createGVisorTun
	}
	if !kernelTunSupported {
		errors.LogWarning(context.Background(), "Using gVisor TUN. Kernel TUN is not supported on your OS, or your permission is insufficient.")
		return createGVisorTun
	}
	errors.LogWarning(context.Background(), "Using kernel TUN.")
	return createKernelTun
}
