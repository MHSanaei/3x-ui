package service

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
)

const (
	// Traffic control: 1 Mbps = 1000 kbit/s.
	KbitPerMbps = 1000

	panelHTBHandle       = "30a1:"
	panelHTBMajor        = "30a1"
	panelDefaultClassID  = "30a1:999"
	panelUnlimitedRate   = "1000000mbit"
	panelIFBDevice       = "ifb3xui0"
	panelIngressPrefBase = 49152
	panelIngressPrefSpan = 2048
)

type tcSelector struct {
	dev        string
	protocol   string
	port       int
	mbps       int
	listenCIDR string
}

type localInterface struct {
	name  string
	up    bool
	addrs []net.IP
}

func (s tcSelector) key() string {
	return strings.Join([]string{s.dev, s.protocol, s.listenCIDR, strconv.Itoa(s.port)}, "|")
}

func (s tcSelector) logLabel() string {
	listen := s.listenCIDR
	if listen == "" {
		listen = "*"
	}
	return fmt.Sprintf("%s/%s %s:%d=%dMbps", s.dev, s.protocol, listen, s.port, s.mbps)
}

func isValidDeviceName(dev string) bool {
	if len(dev) == 0 || len(dev) > 15 {
		return false
	}
	for _, r := range dev {
		if !(r >= 'a' && r <= 'z') &&
			!(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') &&
			r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
}

func isWildcardListen(listen string) bool {
	switch strings.TrimSpace(listen) {
	case "", "0.0.0.0", "::", "::0":
		return true
	default:
		return false
	}
}

func normalizeListenAddress(listen string) string {
	listen = strings.TrimSpace(listen)
	listen = strings.TrimPrefix(listen, "[")
	listen = strings.TrimSuffix(listen, "]")
	if idx := strings.IndexByte(listen, '%'); idx >= 0 {
		listen = listen[:idx]
	}
	return listen
}

func cidrForIP(ip net.IP) string {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4.String() + "/32"
	}
	return ip.String() + "/128"
}

func matchFamilyKeyword(protocol string) string {
	if protocol == "ipv6" {
		return "ip6"
	}
	return "ip"
}

func buildClassID(minor int) string {
	return fmt.Sprintf("%s:%d", panelHTBMajor, minor)
}

func mbpsToKbit(mbps int) int {
	if mbps <= 0 {
		return 0
	}
	return mbps * KbitPerMbps
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
		outStr = strings.TrimSpace(outStr + "\n" + errStr)
	}
	return outStr, nil
}

func discoverLocalInterfaces() ([]localInterface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	result := make([]localInterface, 0, len(ifaces))
	for _, iface := range ifaces {
		if !isValidDeviceName(iface.Name) {
			continue
		}
		addrs, addrErr := iface.Addrs()
		if addrErr != nil {
			logger.Debugf("Failed to list addresses for interface %s: %v", iface.Name, addrErr)
		}
		ips := make([]net.IP, 0, len(addrs))
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsUnspecified() {
				continue
			}
			ips = append(ips, ip)
		}
		result = append(result, localInterface{
			name:  iface.Name,
			up:    iface.Flags&net.FlagUp != 0,
			addrs: ips,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].name < result[j].name
	})
	return result, nil
}

func hasProtocolAddress(iface localInterface, protocol string) bool {
	for _, ip := range iface.addrs {
		if protocol == "ip" && ip.To4() != nil {
			return true
		}
		if protocol == "ipv6" && ip.To4() == nil {
			return true
		}
	}
	return false
}

func resolveInboundSelectors(inbound *model.Inbound, ifaces []localInterface) []tcSelector {
	if inbound == nil || inbound.Port <= 0 || inbound.Port > 65535 || inbound.SpeedLimit <= 0 {
		return nil
	}

	if isWildcardListen(inbound.Listen) {
		selectors := make([]tcSelector, 0, len(ifaces)*2)
		seen := map[string]struct{}{}
		for _, iface := range ifaces {
			if !iface.up {
				continue
			}
			if hasProtocolAddress(iface, "ip") {
				selector := tcSelector{dev: iface.name, protocol: "ip", port: inbound.Port, mbps: inbound.SpeedLimit}
				if _, ok := seen[selector.key()]; !ok {
					seen[selector.key()] = struct{}{}
					selectors = append(selectors, selector)
				}
			}
			if hasProtocolAddress(iface, "ipv6") {
				selector := tcSelector{dev: iface.name, protocol: "ipv6", port: inbound.Port, mbps: inbound.SpeedLimit}
				if _, ok := seen[selector.key()]; !ok {
					seen[selector.key()] = struct{}{}
					selectors = append(selectors, selector)
				}
			}
		}
		return selectors
	}

	listen := normalizeListenAddress(inbound.Listen)
	ip := net.ParseIP(listen)
	if ip == nil {
		logger.Warningf("Skip inbound speed limit for %q:%d: listen address is not a local IP", inbound.Listen, inbound.Port)
		return nil
	}

	protocol := "ipv6"
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
		protocol = "ip"
	}

	selectors := make([]tcSelector, 0, 2)
	seen := map[string]struct{}{}
	for _, iface := range ifaces {
		for _, addr := range iface.addrs {
			if !addr.Equal(ip) {
				continue
			}
			selector := tcSelector{
				dev:        iface.name,
				protocol:   protocol,
				port:       inbound.Port,
				mbps:       inbound.SpeedLimit,
				listenCIDR: cidrForIP(ip),
			}
			if _, ok := seen[selector.key()]; ok {
				continue
			}
			seen[selector.key()] = struct{}{}
			selectors = append(selectors, selector)
		}
	}

	if len(selectors) == 0 {
		logger.Warningf("Skip inbound speed limit for %s:%d: no interface owns listen address %s", inbound.Tag, inbound.Port, listen)
	}

	return selectors
}

func normalizedSpeedLimitType(limitType string) string {
	switch strings.ToLower(strings.TrimSpace(limitType)) {
	case "up", "down", "all":
		return strings.ToLower(strings.TrimSpace(limitType))
	default:
		return "all"
	}
}

func buildLimitSelectors(inbounds []*model.Inbound, ifaces []localInterface) ([]tcSelector, []tcSelector) {
	downMap := map[string]tcSelector{}
	upMap := map[string]tcSelector{}

	for _, inbound := range inbounds {
		if inbound == nil || !inbound.Enable {
			continue
		}
		selectors := resolveInboundSelectors(inbound, ifaces)
		if len(selectors) == 0 {
			continue
		}
		switch normalizedSpeedLimitType(inbound.SpeedLimitType) {
		case "down":
			for _, selector := range selectors {
				downMap[selector.key()] = selector
			}
		case "up":
			for _, selector := range selectors {
				upMap[selector.key()] = selector
			}
		default:
			for _, selector := range selectors {
				downMap[selector.key()] = selector
				upMap[selector.key()] = selector
			}
		}
	}

	down := make([]tcSelector, 0, len(downMap))
	for _, selector := range downMap {
		down = append(down, selector)
	}
	up := make([]tcSelector, 0, len(upMap))
	for _, selector := range upMap {
		up = append(up, selector)
	}
	sortSelectors(down)
	sortSelectors(up)
	return down, up
}

func sortSelectors(selectors []tcSelector) {
	sort.Slice(selectors, func(i, j int) bool {
		if selectors[i].dev != selectors[j].dev {
			return selectors[i].dev < selectors[j].dev
		}
		if selectors[i].protocol != selectors[j].protocol {
			return selectors[i].protocol < selectors[j].protocol
		}
		if selectors[i].listenCIDR != selectors[j].listenCIDR {
			return selectors[i].listenCIDR < selectors[j].listenCIDR
		}
		return selectors[i].port < selectors[j].port
	})
}

func formatSelectors(selectors []tcSelector) string {
	if len(selectors) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(selectors))
	for _, selector := range selectors {
		parts = append(parts, selector.logLabel())
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func tcShowQdisc(dev string) string {
	out, err := exec.Command("tc", "qdisc", "show", "dev", dev).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func tcShowFilters(dev string, parent string) string {
	out, err := exec.Command("tc", "filter", "show", "dev", dev, "parent", parent).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func rootQdiscLine(existing string) string {
	for _, line := range strings.Split(existing, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, " root") {
			return line
		}
	}
	return ""
}

func isPanelOwnedRootQdisc(existing string) bool {
	line := rootQdiscLine(existing)
	return strings.Contains(line, fmt.Sprintf("qdisc htb %s root", panelHTBHandle))
}

func shouldTakeOverRootQdisc(existing string) bool {
	line := rootQdiscLine(existing)
	if line == "" {
		return true
	}
	return strings.Contains(line, " 0: root")
}

func cleanupPanelRootQdisc(dev string) {
	existing := tcShowQdisc(dev)
	if !isPanelOwnedRootQdisc(existing) {
		return
	}
	if _, err := runCmd("tc", "qdisc", "del", "dev", dev, "root"); err != nil {
		logger.Debugf("Failed to remove managed root qdisc on %s: %v", dev, err)
	}
}

func ensureIngressQdisc(dev string) error {
	if _, err := runCmd("tc", "qdisc", "add", "dev", dev, "handle", "ffff:", "ingress"); err != nil {
		if strings.Contains(err.Error(), "File exists") || strings.Contains(err.Error(), "Exclusivity flag on") {
			return nil
		}
		return err
	}
	return nil
}

func managedIngressPrefs(dev string) []int {
	output := tcShowFilters(dev, "ffff:")
	if output == "" {
		return nil
	}
	seen := map[int]struct{}{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		for idx := 0; idx < len(fields)-1; idx++ {
			if fields[idx] != "pref" {
				continue
			}
			pref, err := strconv.Atoi(fields[idx+1])
			if err != nil {
				continue
			}
			if pref < panelIngressPrefBase || pref >= panelIngressPrefBase+panelIngressPrefSpan {
				continue
			}
			seen[pref] = struct{}{}
		}
	}
	prefs := make([]int, 0, len(seen))
	for pref := range seen {
		prefs = append(prefs, pref)
	}
	sort.Ints(prefs)
	return prefs
}

func cleanupManagedIngressFilters(dev string) {
	for _, pref := range managedIngressPrefs(dev) {
		if _, err := runCmd("tc", "filter", "del", "dev", dev, "parent", "ffff:", "pref", strconv.Itoa(pref)); err != nil {
			logger.Debugf("Failed to remove managed ingress filter pref=%d on %s: %v", pref, dev, err)
		}
	}
}

func ensureIFBUp(ifb string) error {
	if _, err := runCmd("ip", "link", "show", ifb); err != nil {
		if _, err = runCmd("ip", "link", "add", ifb, "type", "ifb"); err != nil {
			return err
		}
	}
	_, err := runCmd("ip", "link", "set", "dev", ifb, "up")
	return err
}

func cleanupIFBRoot(ifb string) {
	existing := tcShowQdisc(ifb)
	if !isPanelOwnedRootQdisc(existing) {
		return
	}
	if _, err := runCmd("tc", "qdisc", "del", "dev", ifb, "root"); err != nil {
		logger.Debugf("Failed to remove managed IFB root qdisc on %s: %v", ifb, err)
	}
}

func addHTBClass(dev string, classID string, mbps int) error {
	rate := fmt.Sprintf("%dkbit", mbpsToKbit(mbps))
	_, err := runCmd("tc", "class", "replace", "dev", dev, "parent", panelHTBHandle, "classid", classID, "htb", "rate", rate, "ceil", rate)
	return err
}

func addHTBFlowFilter(dev string, parent string, selector tcSelector, classID string, ipDirection string, portField string) error {
	matchFamily := matchFamilyKeyword(selector.protocol)
	args := []string{"filter", "add", "dev", dev, "protocol", selector.protocol, "parent", parent, "prio", "1", "u32"}
	if selector.listenCIDR != "" {
		args = append(args, "match", matchFamily, ipDirection, selector.listenCIDR)
	}
	args = append(args, "match", matchFamily, portField, strconv.Itoa(selector.port), "0xffff", "flowid", classID)
	_, err := runCmd("tc", args...)
	if err != nil && selector.protocol == "ipv6" {
		logger.Debugf("IPv6 tc filter not added on %s for %s: %v", dev, selector.logLabel(), err)
		return nil
	}
	return err
}

func applyHTBEgressLimit(dev string, selectors []tcSelector) (err error) {
	if len(selectors) == 0 {
		return nil
	}

	existing := tcShowQdisc(dev)
	if existing != "" && !isPanelOwnedRootQdisc(existing) && !shouldTakeOverRootQdisc(existing) {
		return fmt.Errorf("refuse to override existing root qdisc on %s: %s", dev, strings.TrimSpace(rootQdiscLine(existing)))
	}

	defer func() {
		if err != nil {
			cleanupPanelRootQdisc(dev)
		}
	}()

	if rootQdiscLine(existing) != "" {
		if _, delErr := runCmd("tc", "qdisc", "del", "dev", dev, "root"); delErr != nil {
			logger.Debugf("Failed to remove previous root qdisc on %s before apply: %v", dev, delErr)
		}
	}

	if _, err = runCmd("tc", "qdisc", "replace", "dev", dev, "root", "handle", panelHTBHandle, "htb", "default", "999"); err != nil {
		return err
	}
	if _, err = runCmd("tc", "class", "replace", "dev", dev, "parent", panelHTBHandle, "classid", panelDefaultClassID, "htb", "rate", panelUnlimitedRate, "ceil", panelUnlimitedRate); err != nil {
		return err
	}

	for idx, selector := range selectors {
		classID := buildClassID(10 + idx)
		if err = addHTBClass(dev, classID, selector.mbps); err != nil {
			return err
		}
		if err = addHTBFlowFilter(dev, panelHTBHandle, selector, classID, "src", "sport"); err != nil {
			return err
		}
	}
	return nil
}

func groupSelectorsByDev(selectors []tcSelector) map[string][]tcSelector {
	grouped := make(map[string][]tcSelector)
	for _, selector := range selectors {
		grouped[selector.dev] = append(grouped[selector.dev], selector)
	}
	for dev := range grouped {
		sortSelectors(grouped[dev])
	}
	return grouped
}

func reconcileEgress(ifaces []localInterface, selectors []tcSelector) error {
	grouped := groupSelectorsByDev(selectors)
	for _, iface := range ifaces {
		if _, ok := grouped[iface.name]; ok {
			continue
		}
		cleanupPanelRootQdisc(iface.name)
	}
	for dev, devSelectors := range grouped {
		if err := applyHTBEgressLimit(dev, devSelectors); err != nil {
			return err
		}
	}
	return nil
}

func addIngressRedirectFilter(dev string, ifb string, selector tcSelector, pref int) error {
	matchFamily := matchFamilyKeyword(selector.protocol)
	args := []string{"filter", "add", "dev", dev, "parent", "ffff:", "protocol", selector.protocol, "pref", strconv.Itoa(pref), "u32"}
	if selector.listenCIDR != "" {
		args = append(args, "match", matchFamily, "dst", selector.listenCIDR)
	}
	args = append(args, "match", matchFamily, "dport", strconv.Itoa(selector.port), "0xffff", "action", "mirred", "egress", "redirect", "dev", ifb)
	_, err := runCmd("tc", args...)
	if err != nil && selector.protocol == "ipv6" {
		logger.Debugf("IPv6 ingress redirect not added on %s for %s: %v", dev, selector.logLabel(), err)
		return nil
	}
	return err
}

func addIngressPoliceFilter(dev string, selector tcSelector, pref int) error {
	matchFamily := matchFamilyKeyword(selector.protocol)
	rate := fmt.Sprintf("%dkbit", mbpsToKbit(selector.mbps))
	args := []string{"filter", "add", "dev", dev, "parent", "ffff:", "protocol", selector.protocol, "pref", strconv.Itoa(pref), "u32"}
	if selector.listenCIDR != "" {
		args = append(args, "match", matchFamily, "dst", selector.listenCIDR)
	}
	args = append(args, "match", matchFamily, "dport", strconv.Itoa(selector.port), "0xffff", "police", "rate", rate, "burst", "32k", "drop", "flowid", ":1")
	_, err := runCmd("tc", args...)
	if err != nil && selector.protocol == "ipv6" {
		logger.Debugf("IPv6 ingress police not added on %s for %s: %v", dev, selector.logLabel(), err)
		return nil
	}
	return err
}

func applyIFBRootLimit(ifb string, selectors []tcSelector) (err error) {
	defer func() {
		if err != nil {
			cleanupIFBRoot(ifb)
		}
	}()

	if rootQdiscLine(tcShowQdisc(ifb)) != "" {
		if _, delErr := runCmd("tc", "qdisc", "del", "dev", ifb, "root"); delErr != nil {
			logger.Debugf("Failed to remove previous IFB root qdisc on %s before apply: %v", ifb, delErr)
		}
	}

	if _, err = runCmd("tc", "qdisc", "replace", "dev", ifb, "root", "handle", panelHTBHandle, "htb", "default", "999"); err != nil {
		return err
	}
	if _, err = runCmd("tc", "class", "replace", "dev", ifb, "parent", panelHTBHandle, "classid", panelDefaultClassID, "htb", "rate", panelUnlimitedRate, "ceil", panelUnlimitedRate); err != nil {
		return err
	}

	for idx, selector := range selectors {
		classID := buildClassID(10 + idx)
		if err = addHTBClass(ifb, classID, selector.mbps); err != nil {
			return err
		}
		if err = addHTBFlowFilter(ifb, panelHTBHandle, selector, classID, "dst", "dport"); err != nil {
			return err
		}
	}
	return nil
}

func applyIngressRedirects(selectors []tcSelector, ifaces []localInterface) error {
	if err := ensureIFBUp(panelIFBDevice); err != nil {
		return err
	}
	if err := applyIFBRootLimit(panelIFBDevice, selectors); err != nil {
		return err
	}

	grouped := groupSelectorsByDev(selectors)
	for _, iface := range ifaces {
		cleanupManagedIngressFilters(iface.name)
	}
	for dev, devSelectors := range grouped {
		if err := ensureIngressQdisc(dev); err != nil {
			return err
		}
		for idx, selector := range devSelectors {
			pref := panelIngressPrefBase + idx
			if err := addIngressRedirectFilter(dev, panelIFBDevice, selector, pref); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyIngressPolice(selectors []tcSelector, ifaces []localInterface) error {
	grouped := groupSelectorsByDev(selectors)
	for _, iface := range ifaces {
		cleanupManagedIngressFilters(iface.name)
	}
	for dev, devSelectors := range grouped {
		if err := ensureIngressQdisc(dev); err != nil {
			return err
		}
		for idx, selector := range devSelectors {
			pref := panelIngressPrefBase + idx
			if err := addIngressPoliceFilter(dev, selector, pref); err != nil {
				return err
			}
		}
	}
	return nil
}

func reconcileIngress(ifaces []localInterface, selectors []tcSelector) (string, error) {
	for _, iface := range ifaces {
		if len(selectors) == 0 {
			cleanupManagedIngressFilters(iface.name)
		}
	}
	if len(selectors) == 0 {
		cleanupIFBRoot(panelIFBDevice)
		return "none", nil
	}

	if err := applyIngressRedirects(selectors, ifaces); err != nil {
		if strings.Contains(err.Error(), "Unknown device type") || strings.Contains(err.Error(), "Operation not supported") {
			logger.Infof("Uplink speed limit fallback to ingress policing (ifb unavailable): %v", err)
			cleanupIFBRoot(panelIFBDevice)
			if err2 := applyIngressPolice(selectors, ifaces); err2 != nil {
				return "", err2
			}
			return "police", nil
		}
		return "", err
	}
	return "ifb", nil
}

func applyInboundPortSpeedLimitWithTC(inbounds []*model.Inbound) error {
	if runtime.GOOS != "linux" {
		logger.Info("Inbound speed limit via tc is only supported on Linux; skipping")
		return nil
	}
	if _, err := exec.LookPath("tc"); err != nil {
		logger.Warning("Speed limit via tc requested but tc not found in PATH; skipping traffic control")
		return nil
	}

	ifaces, err := discoverLocalInterfaces()
	if err != nil {
		return err
	}
	downSelectors, upSelectors := buildLimitSelectors(inbounds, ifaces)

	logger.Debugf("Reconciling inbound speed limits via tc: down=%s up=%s", formatSelectors(downSelectors), formatSelectors(upSelectors))

	if err := reconcileEgress(ifaces, downSelectors); err != nil {
		return err
	}
	uplinkMode, err := reconcileIngress(ifaces, upSelectors)
	if err != nil {
		return err
	}

	logger.Infof("Inbound speed limit (tc) reconciled: down_rules=%d up_rules=%d uplink_mode=%s", len(downSelectors), len(upSelectors), uplinkMode)
	return nil
}

// ApplyInboundPortSpeedLimits applies inbound-level speed limits using Linux tc.
func (s *XrayService) ApplyInboundPortSpeedLimits() {
	if s == nil {
		logger.Warning("Apply inbound speed limit: XrayService is nil")
		return
	}
	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Apply inbound speed limit failed to list inbounds:", err)
		return
	}
	if err := applyInboundPortSpeedLimitWithTC(inbounds); err != nil {
		logger.Warning("Apply inbound speed limit (tc) failed:", err)
	}
}
