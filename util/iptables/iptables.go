// Package iptables manages a dedicated iptables chain (3X-UI-BLOCK) used to
// drop traffic from clients that have exceeded their bandwidth or time limits.
// All rules are inserted into the custom chain so they are isolated from
// OS/admin firewall rules and can be enumerated or flushed independently.
package iptables

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
)

const chain = "3X-UI-BLOCK"

// EnsureChain creates the custom chain if it does not exist and adds a jump
// rule from INPUT so the chain is evaluated for every incoming packet.
func EnsureChain() error {
	// Create chain — ignore "already exists" error
	out, err := run("iptables", "-N", chain)
	if err != nil && !strings.Contains(out+err.Error(), "already exists") {
		return fmt.Errorf("iptables -N %s: %w (%s)", chain, err, out)
	}

	// Idempotent: only add the jump rule if it is not already present
	_, checkErr := run("iptables", "-C", "INPUT", "-j", chain)
	if checkErr != nil {
		_, err = run("iptables", "-I", "INPUT", "-j", chain)
		if err != nil {
			return fmt.Errorf("iptables -I INPUT -j %s: %w", chain, err)
		}
	}
	return nil
}

// FlushChain removes all rules from the custom chain. Used on startup to
// clear any stale rules left over from a previous crash.
func FlushChain() error {
	_, err := run("iptables", "-F", chain)
	if err != nil {
		return fmt.Errorf("iptables -F %s: %w", chain, err)
	}
	return nil
}

// BlockIP inserts a DROP rule for the given source IP on the given TCP destination
// port into the custom chain. The comment embeds the current Unix timestamp so
// the rule can be age-checked later. Duplicate rules are skipped.
func BlockIP(ip string, port int) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	// Skip if an identical rule already exists
	_, err := run("iptables", "-C", chain,
		"-s", ip,
		"-p", "tcp", "--dport", strconv.Itoa(port),
		"-j", "DROP")
	if err == nil {
		return nil
	}

	comment := fmt.Sprintf("3xui:block:%d", time.Now().Unix())
	_, err = run("iptables", "-I", chain,
		"-s", ip,
		"-p", "tcp", "--dport", strconv.Itoa(port),
		"-m", "comment", "--comment", comment,
		"-j", "DROP")
	if err != nil {
		return fmt.Errorf("iptables BlockIP %s:%d: %w", ip, port, err)
	}
	return nil
}

// UnblockIP removes the DROP rule for the given source IP and TCP destination port.
func UnblockIP(ip string, port int) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}
	_, err := run("iptables", "-D", chain,
		"-s", ip,
		"-p", "tcp", "--dport", strconv.Itoa(port),
		"-m", "comment", "--comment", findComment(ip, port),
		"-j", "DROP")
	if err != nil {
		// Fall back: delete without the comment (handles rules added without matching comment)
		_, err = run("iptables", "-D", chain,
			"-s", ip,
			"-p", "tcp", "--dport", strconv.Itoa(port),
			"-j", "DROP")
		if err != nil {
			return fmt.Errorf("iptables UnblockIP %s:%d: %w", ip, port, err)
		}
	}
	return nil
}

// RuleEntry represents a parsed rule from the custom chain.
type RuleEntry struct {
	IP         string
	Port       int
	InsertedAt int64 // Unix timestamp from the comment, 0 if not present
}

// ListRules parses all rules in the custom chain and returns structured entries.
func ListRules() ([]RuleEntry, error) {
	out, err := runOutput("iptables", "-S", chain)
	if err != nil {
		return nil, fmt.Errorf("iptables -S %s: %w", chain, err)
	}
	var rules []RuleEntry
	for _, line := range strings.Split(out, "\n") {
		entry, ok := parseLine(line)
		if ok {
			rules = append(rules, entry)
		}
	}
	return rules, nil
}

// parseLine extracts IP, port, and timestamp from a single `-S` output line.
// Example line:
//
//	-A 3X-UI-BLOCK -s 1.2.3.4/32 -p tcp -m tcp --dport 443 -m comment --comment 3xui:block:1700000000 -j DROP
func parseLine(line string) (RuleEntry, bool) {
	if !strings.Contains(line, "-j DROP") {
		return RuleEntry{}, false
	}
	var entry RuleEntry

	parts := strings.Fields(line)
	for i, p := range parts {
		switch p {
		case "-s":
			if i+1 < len(parts) {
				entry.IP = strings.TrimSuffix(parts[i+1], "/32")
			}
		case "--dport":
			if i+1 < len(parts) {
				if v, err := strconv.Atoi(parts[i+1]); err == nil {
					entry.Port = v
				}
			}
		case "--comment":
			if i+1 < len(parts) {
				comment := parts[i+1]
				// format: 3xui:block:<timestamp>
				if strings.HasPrefix(comment, "3xui:block:") {
					ts, err := strconv.ParseInt(strings.TrimPrefix(comment, "3xui:block:"), 10, 64)
					if err == nil {
						entry.InsertedAt = ts
					}
				}
			}
		}
	}
	if entry.IP == "" || entry.Port == 0 {
		return RuleEntry{}, false
	}
	return entry, true
}

// findComment retrieves the --comment value for an existing rule matching ip:port.
// Returns an empty string if not found (caller will delete without comment).
func findComment(ip string, port int) string {
	out, err := runOutput("iptables", "-S", chain)
	if err != nil {
		return ""
	}
	needle := fmt.Sprintf("-s %s/32", ip)
	dport := fmt.Sprintf("--dport %d", port)
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, needle) && strings.Contains(line, dport) {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "--comment" && i+1 < len(parts) {
					return parts[i+1]
				}
			}
		}
	}
	return ""
}

// run executes an iptables command and returns combined output and error.
func run(name string, args ...string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		logger.Warning("iptables not found in PATH:", err)
		return "", err
	}
	cmd := exec.Command(path, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runOutput executes a command and returns stdout output.
func runOutput(name string, args ...string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	out, err := exec.Command(path, args...).Output()
	return string(out), err
}
