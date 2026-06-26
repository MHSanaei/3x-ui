// Package portforward provides utilities for generating firewall port forwarding rules
// for Hysteria2 UDP port hopping configuration.
package portforward

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// PortRange represents a range of ports
type PortRange struct {
	Start int
	End   int
}

// ParsePortRange parses a port range string in format "start-end"
func ParsePortRange(rangeStr string) (*PortRange, error) {
	parts := strings.Split(strings.TrimSpace(rangeStr), "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid port range format: expected 'start-end', got '%s'", rangeStr)
	}

	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || start <= 0 || start > 65535 {
		return nil, fmt.Errorf("invalid start port: %s", parts[0])
	}

	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || end <= 0 || end > 65535 {
		return nil, fmt.Errorf("invalid end port: %s", parts[1])
	}

	if start > end {
		return nil, fmt.Errorf("start port (%d) cannot be greater than end port (%d)", start, end)
	}

	return &PortRange{Start: start, End: end}, nil
}

// FirewallType defines the type of firewall
type FirewallType string

const (
	FirewallIptables   FirewallType = "iptables"
	FirewallUfw        FirewallType = "ufw"
	FirewallFirewalld  FirewallType = "firewalld"
	FirewallNftables   FirewallType = "nftables"
)

// Rule represents a single firewall rule
type Rule struct {
	Command string
	Comment string
}

// RuleSet represents a set of firewall rules
type RuleSet struct {
	FirewallType FirewallType
	Rules        []Rule
}

// Generator generates firewall rules for port forwarding
type Generator struct {
	BasePort  int
	PortRange *PortRange
	RuleType  FirewallType
}

// NewGenerator creates a new port forwarding rule generator
func NewGenerator(basePort int, portRangeStr string, ruleType FirewallType) (*Generator, error) {
	if basePort <= 0 || basePort > 65535 {
		return nil, fmt.Errorf("invalid base port: %d", basePort)
	}

	pr, err := ParsePortRange(portRangeStr)
	if err != nil {
		return nil, err
	}

	return &Generator{
		BasePort:  basePort,
		PortRange: pr,
		RuleType:  ruleType,
	}, nil
}

// GenerateIptables generates iptables rules for UDP port forwarding
func (g *Generator) GenerateIptables() *RuleSet {
	rules := []Rule{}
	rules = append(rules, Rule{
		Comment: "Flush existing UDP port hopping rules (optional - uncomment if needed)",
		Command: "# iptables -t nat -D PREROUTING -p udp -j HYSTERIA_PORT_HOP 2>/dev/null || true",
	})
	rules = append(rules, Rule{
		Comment: "Create new chain for UDP port hopping",
		Command: "iptables -t nat -N HYSTERIA_PORT_HOP 2>/dev/null || true",
	})
	rules = append(rules, Rule{
		Comment: "IPv6 version",
		Command: "ip6tables -t nat -N HYSTERIA_PORT_HOP 2>/dev/null || true",
	})

	// Add rules for each port in the range
	for port := g.PortRange.Start; port <= g.PortRange.End; port++ {
		rules = append(rules, Rule{
			Command: fmt.Sprintf("iptables -t nat -A HYSTERIA_PORT_HOP -p udp --dport %d -j REDIRECT --to-port %d", port, g.BasePort),
		})
		rules = append(rules, Rule{
			Command: fmt.Sprintf("ip6tables -t nat -A HYSTERIA_PORT_HOP -p udp --dport %d -j REDIRECT --to-port %d", port, g.BasePort),
		})
	}

	// Add jump rule to the main PREROUTING chain
	rules = append(rules, Rule{
		Comment: "Jump to custom chain from PREROUTING",
		Command: "iptables -t nat -I PREROUTING 1 -j HYSTERIA_PORT_HOP",
	})
	rules = append(rules, Rule{
		Command: "ip6tables -t nat -I PREROUTING 1 -j HYSTERIA_PORT_HOP",
	})

	// Save rules
	rules = append(rules, Rule{
		Comment: "Save rules permanently (Debian/Ubuntu)",
		Command: "iptables-save > /etc/iptables/rules.v4",
	})
	rules = append(rules, Rule{
		Command: "ip6tables-save > /etc/iptables/rules.v6",
	})

	return &RuleSet{
		FirewallType: FirewallIptables,
		Rules:        rules,
	}
}

// GenerateUfw generates UFW rules for UDP port forwarding
func (g *Generator) GenerateUfw() *RuleSet {
	rules := []Rule{}
	rules = append(rules, Rule{
		Comment: "Allow base port",
		Command: fmt.Sprintf("ufw allow %d/udp", g.BasePort),
	})

	rules = append(rules, Rule{
		Comment: "Allow ports in the hopping range",
		Command: fmt.Sprintf("ufw allow %d:%d/udp", g.PortRange.Start, g.PortRange.End),
	})

	rules = append(rules, Rule{
		Comment: "Enable UFW if not already enabled",
		Command: "ufw enable",
	})

	return &RuleSet{
		FirewallType: FirewallUfw,
		Rules:        rules,
	}
}

// GenerateFirewalld generates firewalld rules for UDP port forwarding
func (g *Generator) GenerateFirewalld() *RuleSet {
	rules := []Rule{}
	rules = append(rules, Rule{
		Comment: "Add service zone",
		Command: "firewall-cmd --permanent --new-service=hysteria2 2>/dev/null || true",
	})

	rules = append(rules, Rule{
		Comment: "Set service port",
		Command: fmt.Sprintf("firewall-cmd --permanent --service=hysteria2 --set-port=%d:udp", g.BasePort),
	})

	rules = append(rules, Rule{
		Comment: "Add service to public zone",
		Command: "firewall-cmd --permanent --zone=public --add-service=hysteria2",
	})

	// Add rules for port forwarding
	for port := g.PortRange.Start; port <= g.PortRange.End; port++ {
		if port != g.BasePort {
			rules = append(rules, Rule{
				Command: fmt.Sprintf("firewall-cmd --permanent --zone=public --add-forward-port=port=%d:proto=udp:toport=%d", port, g.BasePort),
			})
		}
	}

	rules = append(rules, Rule{
		Comment: "Reload firewall to apply rules",
		Command: "firewall-cmd --reload",
	})

	return &RuleSet{
		FirewallType: FirewallFirewalld,
		Rules:        rules,
	}
}

// GenerateNftables generates nftables rules for UDP port forwarding
func (g *Generator) GenerateNftables() *RuleSet {
	rules := []Rule{}

	// Build port list
	var portList []string
	for port := g.PortRange.Start; port <= g.PortRange.End; port++ {
		portList = append(portList, fmt.Sprintf("%d", port))
	}
	ports := strings.Join(portList, ", ")

	rules = append(rules, Rule{
		Comment: "Create table if it doesn't exist",
		Command: "nft add table ip nat 2>/dev/null || true",
	})
	rules = append(rules, Rule{
		Command: "nft add table ip6 nat 2>/dev/null || true",
	})

	rules = append(rules, Rule{
		Comment: "Create chain if it doesn't exist",
		Command: "nft add chain ip nat prerouting '{ type nat hook prerouting priority dstnat; policy accept; }' 2>/dev/null || true",
	})
	rules = append(rules, Rule{
		Command: "nft add chain ip6 nat prerouting '{ type nat hook prerouting priority dstnat; policy accept; }' 2>/dev/null || true",
	})

	rules = append(rules, Rule{
		Comment: "Add redirect rule for IPv4",
		Command: fmt.Sprintf("nft add rule ip nat prerouting udp dport { %s } redirect to %d", ports, g.BasePort),
	})
	rules = append(rules, Rule{
		Comment: "Add redirect rule for IPv6",
		Command: fmt.Sprintf("nft add rule ip6 nat prerouting udp dport { %s } redirect to %d", ports, g.BasePort),
	})

	rules = append(rules, Rule{
		Comment: "Save rules to file for persistence",
		Command: "nft list ruleset > /etc/nftables.conf",
	})

	return &RuleSet{
		FirewallType: FirewallNftables,
		Rules:        rules,
	}
}

// Generate generates port forwarding rules based on the configured firewall type
func (g *Generator) Generate() *RuleSet {
	switch g.RuleType {
	case FirewallUfw:
		return g.GenerateUfw()
	case FirewallFirewalld:
		return g.GenerateFirewalld()
	case FirewallNftables:
		return g.GenerateNftables()
	case FirewallIptables:
		fallthrough
	default:
		return g.GenerateIptables()
	}
}

// ToStringSlice converts the rule set to a slice of command strings
func (rs *RuleSet) ToStringSlice() []string {
	var result []string
	for _, rule := range rs.Rules {
		if rule.Comment != "" {
			result = append(result, fmt.Sprintf("# %s", rule.Comment))
		}
		result = append(result, rule.Command)
	}
	return result
}

// ToString returns the rules as a formatted string
func (rs *RuleSet) ToString() string {
	return strings.Join(rs.ToStringSlice(), "\n")
}

// ValidatePortRange validates that a port range string is valid
func ValidatePortRange(rangeStr string) error {
	if !regexp.MustCompile(`^\d+-\d+$`).MatchString(strings.TrimSpace(rangeStr)) {
		return fmt.Errorf("invalid port range format: expected 'start-end', got '%s'", rangeStr)
	}

	_, err := ParsePortRange(rangeStr)
	return err
}
