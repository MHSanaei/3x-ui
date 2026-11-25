package service

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

func encodeWireguardDevices(devices []model.WireguardDevice) string {
	encoded, _ := json.Marshal(devices)
	return string(encoded)
}

// Attached devices keep their identity while the target inbound may allocate
// addresses from a different configured subnet.
func prepareWireguardDeviceAddresses(client *model.Client, inbound *model.Inbound) {
	if inbound.Protocol != model.WireGuard {
		return
	}
	client.WireguardDevices = model.CloneWireguardDevices(client.WireguardDevices)
	prefix, err := netip.ParsePrefix(explicitWireguardSubnetBase(inbound.Settings))
	if err != nil {
		return
	}
	for i := range client.WireguardDevices {
		for _, address := range client.WireguardDevices[i].AllowedIPs {
			if !prefix.Contains(wireguardHostAddr(address)) {
				client.WireguardDevices[i].AllowedIPs = nil
				break
			}
		}
	}
}

func defaultWireguardDeviceIdentity(device *model.WireguardDevice) error {
	if device.Id == "" {
		device.Id = uuid.NewString()
	}
	if device.PrivateKey == "" && device.PublicKey == "" {
		privateKey, publicKey, err := wgutil.GenerateWireguardKeypair()
		if err != nil {
			return err
		}
		device.PrivateKey, device.PublicKey = privateKey, publicKey
	}
	if device.PrivateKey != "" {
		publicKey, err := wgutil.PublicKeyFromPrivate(device.PrivateKey)
		if err != nil {
			return fmt.Errorf("wireguard: invalid device private key: %w", err)
		}
		derived, _ := wgutil.KeyToHex(publicKey)
		provided, err := wgutil.KeyToHex(device.PublicKey)
		if err != nil || (provided != "" && provided != derived) {
			return fmt.Errorf("wireguard: device public key does not match private key")
		}
		device.PublicKey = publicKey
	}
	return nil
}

// validateWireguardDevices allocates and validates additional devices. The
// caller owns the inbound lock and rechecks occupancy in the serialized writer.
func validateWireguardDevices(settings string, existing []model.Client, client *model.Client, crossUsed map[string]string) error {
	if client.WgDeviceLimit < 0 || client.WgDeviceLimit > model.MaxWireguardDevices {
		return fmt.Errorf("wireguard: device limit must be between 1 and %d", model.MaxWireguardDevices)
	}
	if len(client.WireguardDevices)+1 > model.EffectiveWireguardDeviceLimit(client.WgDeviceLimit) {
		return fmt.Errorf("wireguard: device limit reached (%d)", model.EffectiveWireguardDeviceLimit(client.WgDeviceLimit))
	}
	used := append([]string(nil), client.AllowedIPs...)
	keys := make(map[string]bool)
	primaryKey, _ := wgutil.KeyToHex(client.PublicKey)
	if primaryKey != "" {
		keys[primaryKey] = true
	}
	for _, other := range existing {
		if strings.EqualFold(other.Email, client.Email) {
			continue
		}
		used = append(used, model.WireguardAllowedIPs(other)...)
		peers := model.WireguardClients(other)
		// Disabled devices keep both their keys and addresses reserved.
		for _, device := range other.WireguardDevices {
			if !device.Enable {
				peers = append(peers, model.Client{PublicKey: device.PublicKey})
			}
		}
		for _, peer := range peers {
			if key, err := wgutil.KeyToHex(peer.PublicKey); err == nil && key != "" {
				if key == primaryKey {
					return fmt.Errorf("wireguard: client public key is already in use")
				}
				keys[key] = true
			}
		}
	}
	base := explicitWireguardSubnetBase(settings)
	if base == "" {
		base = wireguardAllocationBase(used, defaultWireguardBase)
	}
	for address := range crossUsed {
		used = append(used, address)
	}
	ids := make(map[string]bool)
	for i := range client.WireguardDevices {
		device := &client.WireguardDevices[i]
		name, err := validWireguardDeviceName(device.Name)
		if err != nil {
			return err
		}
		device.Name = name
		if err := defaultWireguardDeviceIdentity(device); err != nil {
			return err
		}
		if _, err := uuid.Parse(device.Id); err != nil || ids[device.Id] {
			return fmt.Errorf("wireguard: invalid or duplicate device id")
		}
		ids[device.Id] = true
		key, err := wgutil.KeyToHex(device.PublicKey)
		if err != nil || key == "" {
			return fmt.Errorf("wireguard: invalid device public key")
		}
		if keys[key] {
			return fmt.Errorf("wireguard: device public key is already in use")
		}
		keys[key] = true
		if _, err := wgutil.KeyToHex(device.PreSharedKey); err != nil {
			return fmt.Errorf("wireguard: invalid device preshared key: %w", err)
		}
		if len(device.AllowedIPs) == 0 {
			address, err := allocateWireguardAddress(used, base, true)
			if err != nil {
				return err
			}
			device.AllowedIPs = []string{address}
		}
		addresses, err := normalizeWireguardAllowedIPs(device.AllowedIPs)
		if err != nil || len(addresses) == 0 {
			return fmt.Errorf("wireguard: invalid device allowedIPs")
		}
		if entry, taken := wireguardAllowedIPsOverlap(addresses, used); taken != "" {
			return fmt.Errorf("wireguard: device allowedIPs entry %s overlaps %s", entry, taken)
		}
		device.AllowedIPs = addresses
		used = append(used, addresses...)
	}
	return nil
}
