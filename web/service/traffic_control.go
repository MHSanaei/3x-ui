package service

import (
	"bytes"
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
)

type inboundPortLimit struct {
	port int
	kbps int // KB/s, 0 means unlimited
	typ  string
}

func (l inboundPortLimit) normalizedType() string {
	t := strings.ToLower(strings.TrimSpace(l.typ))
	switch t {
	case "up", "down", "all":
		return t
	default:
		return "all"
	}
}

func detectDefaultNetDev() string {
	// Best-effort: try to detect default route device, otherwise fall back to eth0.
	out, err := exec.Command("sh", "-c", "ip route show default 2>/dev/null | awk '{for(i=1;i<=NF;i++){if($i==\"dev\"){print $(i+1); exit}}}'").Output()
	if err == nil {
		dev := strings.TrimSpace(string(out))
		if dev != "" {
			return dev
		}
	}
	return "eth0"
}

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	outStr := strings.TrimSpace(stdout.String())
	errStr := strings.TrimSpace(stderr.String())
	if err != nil {
		if errStr != "" {
			return outStr, fmt.Errorf("%w: %s", err, errStr)
		}
		return outStr, err
	}
	if errStr != "" {
		// Some commands may write warnings to stderr; include them in output for debugging.
		outStr = strings.TrimSpace(outStr + "\n" + errStr)
	}
	return outStr, nil
}

func tcShowQdisc(dev string) string {
	out, err := exec.Command("tc", "qdisc", "show", "dev", dev).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func shouldTakeOverRootQdisc(existing string) bool {
	// We only takeover if the current root qdisc looks like the default (handle 0:).
	// This avoids clobbering user-managed traffic control on the host.
	// In containers, the default is typically: "qdisc noqueue 0: root ...".
	lines := strings.Split(existing, "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if strings.Contains(ln, " root ") {
			return strings.Contains(ln, " 0: root")
		}
	}
	// If we can't detect anything, be conservative.
	return false
}

func kbpsToKbit(kbps int) int {
	if kbps <= 0 {
		return 0
	}
	// KB/s -> bits/s (using KiB: 1024 bytes) -> kbit/s (1000 bits).
	kbit := int(math.Ceil(float64(kbps) * 1024.0 * 8.0 / 1000.0))
	if kbit < 1 {
		kbit = 1
	}
	return kbit
}

func formatPortLimits(m map[int]int) string {
	if len(m) == 0 {
		return "[]"
	}
	ports := make([]int, 0, len(m))
	for p := range m {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, fmt.Sprintf("%d=%dKB/s", p, m[p]))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func applyHTBEgressLimit(dev string, limits map[int]int) error {
	existing := tcShowQdisc(dev)
	if existing != "" && !shouldTakeOverRootQdisc(existing) && !strings.Contains(existing, "qdisc htb 1: root") {
		return fmt.Errorf("refuse to override existing root qdisc on %s: %s", dev, strings.TrimSpace(existing))
	}

	// Start from a clean state (ignore errors if qdisc doesn't exist).
	_, _ = runCmd("tc", "qdisc", "del", "dev", dev, "root")

	// Replace root qdisc with our HTB. Default class is unlimited (1:999).
	if _, err := runCmd("tc", "qdisc", "replace", "dev", dev, "root", "handle", "1:", "htb", "default", "999"); err != nil {
		return err
	}
	// Unlimited default class.
	_, _ = runCmd("tc", "class", "replace", "dev", dev, "parent", "1:", "classid", "1:999", "htb", "rate", "1000000mbit", "ceil", "1000000mbit")

	ports := make([]int, 0, len(limits))
	for p := range limits {
		ports = append(ports, p)
	}
	sort.Ints(ports)

	minor := 10
	for _, port := range ports {
		kbps := limits[port]
		kbit := kbpsToKbit(kbps)
		classid := fmt.Sprintf("1:%d", minor)
		rate := fmt.Sprintf("%dkbit", kbit)
		minor++

		if _, err := runCmd("tc", "class", "replace", "dev", dev, "parent", "1:", "classid", classid, "htb", "rate", rate, "ceil", rate); err != nil {
			return err
		}
		// Downlink (server -> client) packets have source port = inbound port.
		_, _ = runCmd("tc", "filter", "add", "dev", dev, "protocol", "ip", "parent", "1:", "prio", "1",
			"u32", "match", "ip", "sport", strconv.Itoa(port), "0xffff", "flowid", classid)
		// IPv6 best-effort (ignore errors on kernels without u32 ip6 support).
		_, _ = runCmd("tc", "filter", "add", "dev", dev, "protocol", "ipv6", "parent", "1:", "prio", "1",
			"u32", "match", "ip6", "sport", strconv.Itoa(port), "0xffff", "flowid", classid)
	}
	return nil
}

func cleanupHTBEgress(dev string) {
	existing := tcShowQdisc(dev)
	if !strings.Contains(existing, "qdisc htb 1: root") {
		return
	}
	_, _ = runCmd("tc", "qdisc", "del", "dev", dev, "root")
}

func ensureIFBUp(ifb string) error {
	// Create ifb device if missing, then bring it up.
	_, err := runCmd("sh", "-c", fmt.Sprintf("ip link show %s >/dev/null 2>&1 || ip link add %s type ifb", ifb, ifb))
	if err != nil {
		return err
	}
	_, err = runCmd("ip", "link", "set", "dev", ifb, "up")
	return err
}

func applyHTBIngressLimit(dev string, ifb string, limits map[int]int) error {
	if len(limits) == 0 {
		return nil
	}
	if err := ensureIFBUp(ifb); err != nil {
		return err
	}

	// Attach ingress qdisc and redirect selected traffic to ifb.
	_, _ = runCmd("tc", "qdisc", "del", "dev", dev, "ingress")
	if _, err := runCmd("tc", "qdisc", "add", "dev", dev, "handle", "ffff:", "ingress"); err != nil {
		return err
	}

	ports := make([]int, 0, len(limits))
	for p := range limits {
		ports = append(ports, p)
	}
	sort.Ints(ports)

	for _, port := range ports {
		// Uplink (client -> server) packets have destination port = inbound port.
		_, _ = runCmd("tc", "filter", "add", "dev", dev, "parent", "ffff:", "protocol", "ip", "prio", "1",
			"u32", "match", "ip", "dport", strconv.Itoa(port), "0xffff",
			"action", "mirred", "egress", "redirect", "dev", ifb)
		_, _ = runCmd("tc", "filter", "add", "dev", dev, "parent", "ffff:", "protocol", "ipv6", "prio", "1",
			"u32", "match", "ip6", "dport", strconv.Itoa(port), "0xffff",
			"action", "mirred", "egress", "redirect", "dev", ifb)
	}

	// Shape on ifb egress based on dport.
	_, _ = runCmd("tc", "qdisc", "del", "dev", ifb, "root")
	if _, err := runCmd("tc", "qdisc", "replace", "dev", ifb, "root", "handle", "1:", "htb", "default", "999"); err != nil {
		return err
	}
	_, _ = runCmd("tc", "class", "replace", "dev", ifb, "parent", "1:", "classid", "1:999", "htb", "rate", "1000000mbit", "ceil", "1000000mbit")

	minor := 10
	for _, port := range ports {
		kbps := limits[port]
		kbit := kbpsToKbit(kbps)
		classid := fmt.Sprintf("1:%d", minor)
		rate := fmt.Sprintf("%dkbit", kbit)
		minor++

		if _, err := runCmd("tc", "class", "replace", "dev", ifb, "parent", "1:", "classid", classid, "htb", "rate", rate, "ceil", rate); err != nil {
			return err
		}
		_, _ = runCmd("tc", "filter", "add", "dev", ifb, "protocol", "ip", "parent", "1:", "prio", "1",
			"u32", "match", "ip", "dport", strconv.Itoa(port), "0xffff", "flowid", classid)
		_, _ = runCmd("tc", "filter", "add", "dev", ifb, "protocol", "ipv6", "parent", "1:", "prio", "1",
			"u32", "match", "ip6", "dport", strconv.Itoa(port), "0xffff", "flowid", classid)
	}

	return nil
}

func applyIngressPolice(dev string, limits map[int]int) error {
	if len(limits) == 0 {
		return nil
	}

	// "Police" on ingress drops packets above the rate. It's less smooth than shaping via IFB,
	// but works on kernels without the "ifb" device type (common in minimal/container kernels).
	_, _ = runCmd("tc", "qdisc", "del", "dev", dev, "ingress")
	if _, err := runCmd("tc", "qdisc", "add", "dev", dev, "handle", "ffff:", "ingress"); err != nil {
		return err
	}
	_, _ = runCmd("tc", "filter", "del", "dev", dev, "parent", "ffff:")

	ports := make([]int, 0, len(limits))
	for p := range limits {
		ports = append(ports, p)
	}
	sort.Ints(ports)

	for _, port := range ports {
		kbit := kbpsToKbit(limits[port])
		rate := fmt.Sprintf("%dkbit", kbit)
		burst := "32k"

		// Uplink (client -> server) packets have destination port = inbound port.
		if _, err := runCmd("tc", "filter", "add", "dev", dev, "parent", "ffff:", "protocol", "ip", "prio", "1",
			"u32", "match", "ip", "dport", strconv.Itoa(port), "0xffff",
			"police", "rate", rate, "burst", burst, "drop", "flowid", ":1"); err != nil {
			return err
		}
		// IPv6 best-effort (ignore errors on kernels without u32 ip6 support).
		_, _ = runCmd("tc", "filter", "add", "dev", dev, "parent", "ffff:", "protocol", "ipv6", "prio", "1",
			"u32", "match", "ip6", "dport", strconv.Itoa(port), "0xffff",
			"police", "rate", rate, "burst", burst, "drop", "flowid", ":1")
	}

	return nil
}

func applyInboundPortSpeedLimitWithTC(inbounds []*model.Inbound) error {
	if _, err := exec.LookPath("tc"); err != nil {
		logger.Warning("Speed limit via tc requested but tc not found in PATH; skipping traffic control")
		return nil
	}

	down := map[int]int{} // port -> KB/s
	up := map[int]int{}

	for _, inbound := range inbounds {
		if inbound == nil || !inbound.Enable || inbound.Port <= 0 || inbound.Port > 65535 {
			continue
		}
		if inbound.SpeedLimit <= 0 {
			continue
		}
		typ := inboundPortLimit{typ: inbound.SpeedLimitType}.normalizedType()
		switch typ {
		case "down":
			down[inbound.Port] = inbound.SpeedLimit
		case "up":
			up[inbound.Port] = inbound.SpeedLimit
		default: // all
			down[inbound.Port] = inbound.SpeedLimit
			up[inbound.Port] = inbound.SpeedLimit
		}
	}

	dev := detectDefaultNetDev()

	logger.Debugf("Reconciling inbound speed limits via tc on %s: down=%s up=%s", dev, formatPortLimits(down), formatPortLimits(up))

	if len(down) > 0 {
		if err := applyHTBEgressLimit(dev, down); err != nil {
			return err
		}
	} else {
		// Remove our egress qdisc if we previously installed it.
		cleanupHTBEgress(dev)
	}

	uplinkMode := "none"
	if len(up) > 0 {
		if err := applyHTBIngressLimit(dev, "ifb0", up); err != nil {
			// Typical failure in container environments: "Error: Unknown device type." from "ip link add ... type ifb".
			// Fall back to ingress policing so "up/all" still works.
			if strings.Contains(err.Error(), "Unknown device type") {
				logger.Infof("Uplink speed limit fallback to ingress policing (ifb not supported): %v", err)
				if err2 := applyIngressPolice(dev, up); err2 != nil {
					return err2
				}
				uplinkMode = "police"
				// Best-effort cleanup in case a previous run used IFB.
				_, _ = runCmd("tc", "qdisc", "del", "dev", "ifb0", "root")
			} else {
				return err
			}
		} else {
			uplinkMode = "ifb"
		}
	} else {
		// Remove ingress shaping if present.
		_, _ = runCmd("tc", "qdisc", "del", "dev", dev, "ingress")
		_, _ = runCmd("tc", "qdisc", "del", "dev", "ifb0", "root")
	}

	logger.Infof("Inbound speed limit (tc) reconciled on %s: down_ports=%d up_ports=%d uplink_mode=%s", dev, len(down), len(up), uplinkMode)
	return nil
}

// ApplyInboundPortSpeedLimits applies inbound-level speed limits (by port) using OS traffic control (tc).
// This replaces the previous per-client policy bufferSize approach, which is not a real bandwidth limiter.
func (s *XrayService) ApplyInboundPortSpeedLimits() {
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Apply inbound speed limit failed to list inbounds:", err)
		return
	}
	if err := applyInboundPortSpeedLimitWithTC(inbounds); err != nil {
		logger.Warning("Apply inbound speed limit (tc) failed:", err)
	}
}
