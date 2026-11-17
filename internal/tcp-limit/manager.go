// Package tcplimit protects public Xray TCP ports with per-source connection limits.
package tcplimit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const tableName = "xui_tcp_limit"

// Target is one listener. An invalid Address covers wildcard listeners.
type Target struct {
	Address netip.Addr
	Port    uint16
}

func (t Target) String() string {
	if !t.Address.IsValid() {
		return "*:" + strconv.Itoa(int(t.Port))
	}
	return netip.AddrPortFrom(t.Address, t.Port).String()
}

// Manager owns only its isolated nftables table. Identical updates retain the
// dynamic connection counters, including across panel process restarts.
type Manager struct {
	mu      sync.Mutex
	run     func(string, ...string) ([]byte, error)
	current string
	known   bool
}

func NewManager() *Manager { return &Manager{run: runNFT} }

// Apply replaces the managed rules in one nft transaction. Zero disables the
// limit. Ports share one per-source count across IPv4 and IPv6 independently.
func (m *Manager) Apply(limit int, targets []Target) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.apply(limit, targets, false)
}

// Disable requires successful removal, including when an enabled installation
// lost nft or its privileges after the panel restarted.
func (m *Manager) Disable() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.apply(0, nil, true)
}

// Stage applies protection before a listener change and returns its rollback.
// The caller serializes listener/settings changes until it commits or rolls back.
func (m *Manager) Stage(limit int, targets []Target) (func() error, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.known {
		out, err := m.run("", "list", "table", "inet", tableName)
		if err == nil {
			m.current = tableSignature(out)
			m.known = true
		}
	}
	previous := m.current
	if err := m.apply(limit, targets, false); err != nil {
		return nil, err
	}
	return func() error {
		oldLimit, oldTargets, err := parseSignature(previous)
		if err != nil {
			return err
		}
		return m.Apply(oldLimit, oldTargets)
	}, nil
}

func (m *Manager) apply(limit int, targets []Target, strictDisable bool) error {
	if limit < 0 || limit > 1_000_000 {
		return errors.New("TCP connection limit must be between 0 and 1000000")
	}
	targets = slices.Clone(targets)
	for i := range targets {
		if targets[i].Port == 0 {
			return errors.New("TCP connection limit cannot protect port zero")
		}
		targets[i].Address = targets[i].Address.Unmap()
	}
	slices.SortFunc(targets, func(a, b Target) int {
		if a.Port != b.Port {
			return int(a.Port) - int(b.Port)
		}
		return a.Address.Compare(b.Address)
	})
	targets = slices.Compact(targets)
	signature := ""
	if limit > 0 {
		signature = fmt.Sprintf("3x-ui TCP limit %d targets %s", limit, targetList(targets))
	}
	if m.known && m.current == signature {
		return nil
	}
	if !m.known {
		out, err := m.run("", "list", "table", "inet", tableName)
		if err == nil {
			m.current = tableSignature(out)
			if m.current == signature && signature != "" {
				m.known = true
				return nil
			}
		} else if limit == 0 && !strictDisable {
			// Disabled installations do not need nft or NET_ADMIN privileges.
			return nil
		}
	}
	tx := "add table inet " + tableName + "\ndelete table inet " + tableName + "\n"
	if limit > 0 {
		tx += rules(limit, targets, signature)
	}
	if out, err := m.run(tx, "-f", "-"); err != nil {
		return fmt.Errorf("apply TCP connection limit (requires nftables and NET_ADMIN): %w: %s", err, strings.TrimSpace(string(out)))
	}
	m.current, m.known = signature, true
	return nil
}

func tableSignature(data []byte) string {
	// nft 1.0.6 omits table comments from JSON, but preserves them in text.
	for _, line := range strings.Split(string(data), "\n") {
		if comment, ok := strings.CutPrefix(strings.TrimSpace(line), "comment "); ok {
			value, _ := strconv.Unquote(comment)
			return value
		}
	}
	return ""
}

func parseSignature(signature string) (int, []Target, error) {
	if signature == "" {
		return 0, nil, nil
	}
	limitText, targetsText, ok := strings.Cut(strings.TrimPrefix(signature, "3x-ui TCP limit "), " targets ")
	limit, err := strconv.Atoi(limitText)
	if !ok || err != nil {
		return 0, nil, errors.New("invalid managed TCP connection limit signature")
	}
	var targets []Target
	if targetsText == "" {
		return limit, targets, nil
	}
	for _, value := range strings.Split(targetsText, ", ") {
		if p, ok := strings.CutPrefix(value, "*:"); ok {
			port, err := strconv.ParseUint(p, 10, 16)
			if err != nil {
				return 0, nil, err
			}
			targets = append(targets, Target{Port: uint16(port)})
		} else {
			addr, err := netip.ParseAddrPort(value)
			if err != nil {
				return 0, nil, err
			}
			targets = append(targets, Target{Address: addr.Addr(), Port: addr.Port()})
		}
	}
	return limit, targets, nil
}

func targetList(targets []Target) string {
	values := make([]string, len(targets))
	for i, target := range targets {
		values[i] = target.String()
	}
	return strings.Join(values, ", ")
}

func rules(limit int, targets []Target, signature string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `table inet %s {
 comment %q
 set ipv4_sources { type ipv4_addr; flags dynamic; size 65535; }
 set ipv6_sources { type ipv6_addr; flags dynamic; size 65535; }
 chain input {
  type filter hook input priority -20; policy accept;
  iifname "lo" return
`, tableName, signature)
	for _, target := range targets {
		for _, family := range []string{"ip", "ip6"} {
			selector := ""
			if target.Address.IsValid() {
				if (family == "ip") != target.Address.Is4() {
					continue
				}
				if !target.Address.IsUnspecified() {
					selector = family + " daddr " + target.Address.String() + " "
				}
			}
			set := "ipv4_sources"
			if family == "ip6" {
				set = "ipv6_sources"
			}
			fmt.Fprintf(&b, "  %stcp dport %d ct state new tcp flags & (syn | ack) == syn add @%s { %s saddr ct count over %d } counter reject with tcp reset\n", selector, target.Port, set, family, limit)
		}
	}
	b.WriteString(" }\n}\n")
	return b.String()
}

func runNFT(stdin string, args ...string) ([]byte, error) {
	if runtime.GOOS != "linux" {
		return nil, errors.New("TCP connection limits require Linux nftables")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "nft", args...)
	cmd.Stdin = bytes.NewBufferString(stdin)
	return cmd.CombinedOutput()
}
