package amneziawg

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// Default constants for AWG configuration.
const (
	DefaultSubnetIP     = "10.8.1.1"
	DefaultSubnetCIDR   = 24
	DefaultServerPort   = 55424
	DefaultPrimaryDNS   = "8.8.8.8"
	DefaultSecondaryDNS = "8.8.4.4"
)

// randRange returns a random integer in [min, max].
func randRange(min, max int64) int64 {
	n, err := rand.Int(rand.Reader, big.NewInt(max-min+1))
	if err != nil {
		return min
	}
	return n.Int64() + min
}

// GenerateAWGParams generates random AWG obfuscation parameters.
func GenerateAWGParams() ServerConfig {
	jc := int(randRange(4, 6))
	jmin := 10
	jmax := 50

	s1 := int(randRange(15, 1500))
	s2 := int(randRange(15, 1500))
	for s2 == s1 {
		s2 = int(randRange(15, 1500))
	}
	s3 := int(randRange(10, 500))
	for s3 == s1 || s3 == s2 {
		s3 = int(randRange(10, 500))
	}
	s4 := int(randRange(5, 200))
	for s4 == s1 || s4 == s2 || s4 == s3 {
		s4 = int(randRange(5, 200))
	}

	h1, h2, h3, h4 := generateRangeHeaders()

	return ServerConfig{
		Jc:           jc,
		Jmin:         jmin,
		Jmax:         jmax,
		S1:           s1,
		S2:           s2,
		S3:           s3,
		S4:           s4,
		H1:           h1,
		H2:           h2,
		H3:           h3,
		H4:           h4,
		SubnetIP:     DefaultSubnetIP,
		SubnetCIDR:   DefaultSubnetCIDR,
		ServerPort:   DefaultServerPort,
		PrimaryDNS:   DefaultPrimaryDNS,
		SecondaryDNS: DefaultSecondaryDNS,
	}
}

// generateRangeHeaders generates four sequential non-overlapping range-format
// magic headers for AWG obfuscation.
func generateRangeHeaders() (string, string, string, string) {
	var min int64 = 5
	maxVal := int64(2147483647)

	first := randRange(min, maxVal)
	second := randRange(first, maxVal)
	h1 := fmt.Sprintf("%d-%d", first, second)
	min = second

	first = randRange(min, maxVal)
	second = randRange(first, maxVal)
	h2 := fmt.Sprintf("%d-%d", first, second)
	min = second

	first = randRange(min, maxVal)
	second = randRange(first, maxVal)
	h3 := fmt.Sprintf("%d-%d", first, second)
	min = second

	first = randRange(min, maxVal)
	second = randRange(first, maxVal)
	h4 := fmt.Sprintf("%d-%d", first, second)

	return h1, h2, h3, h4
}

// NextClientIP calculates the next available client IP from a subnet base
// and a list of already-assigned IPs.
func NextClientIP(subnetIP string, assignedIPs []string) (string, error) {
	parts, err := parseIPParts(subnetIP)
	if err != nil {
		return "", err
	}
	baseInt := (int64(parts[0]) << 24) | (int64(parts[1]) << 16) | (int64(parts[2]) << 8) | int64(parts[3])

	used := make(map[int64]bool)
	for _, ip := range assignedIPs {
		p, err := parseIPParts(ip)
		if err != nil {
			continue
		}
		ipInt := (int64(p[0]) << 24) | (int64(p[1]) << 16) | (int64(p[2]) << 8) | int64(p[3])
		used[ipInt-baseInt] = true
	}

	highest := int64(0)
	// Reserve the .1 address for the server when the subnet base is the
	// network address (e.g. 10.8.1.0 → server at .1, clients from .2).
	if strings.HasSuffix(subnetIP, ".0") {
		highest = 1
	}
	for offset := range used {
		if offset > highest {
			highest = offset
		}
	}
	nextOffset := highest + 1
	if nextOffset >= 254 {
		return "", fmt.Errorf("no more IPs available in the subnet")
	}
	nextInt := baseInt + nextOffset
	return fmt.Sprintf("%d.%d.%d.%d",
		(nextInt>>24)&0xFF,
		(nextInt>>16)&0xFF,
		(nextInt>>8)&0xFF,
		nextInt&0xFF,
	), nil
}

func parseIPParts(ip string) ([]int64, error) {
	var a, b, c, d int64
	if _, err := fmt.Sscanf(ip, "%d.%d.%d.%d", &a, &b, &c, &d); err != nil {
		return nil, err
	}
	return []int64{a, b, c, d}, nil
}
