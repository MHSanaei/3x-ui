package sub

import (
	"crypto/rand"
	"encoding/binary"
	"math/big"
	"net"
	"strings"
)

// IsCIDR reports whether s represents a valid CIDR network notation.
func IsCIDR(s string) bool {
	s = strings.Trim(strings.TrimSpace(s), "[]")
	if !strings.Contains(s, "/") {
		return false
	}
	_, _, err := net.ParseCIDR(s)
	return err == nil
}

// RandomIPsFromCIDR returns count unique random host IPs from the specified CIDR.
// Supported across IPv4 and IPv6; caps at the usable address space when smaller.
func RandomIPsFromCIDR(cidr string, count int) ([]string, error) {
	if count <= 0 {
		count = 1
	}
	clean := strings.Trim(strings.TrimSpace(cidr), "[]")
	_, ipNet, err := net.ParseCIDR(clean)
	if err != nil {
		return nil, err
	}

	if ip4 := ipNet.IP.To4(); ip4 != nil {
		return randomIPv4FromCIDR(ip4, ipNet.Mask, count)
	}
	return randomIPv6FromCIDR(ipNet.IP.To16(), ipNet.Mask, count)
}

func randomIPv4FromCIDR(ip4 net.IP, mask net.IPMask, count int) ([]string, error) {
	ones, bits := mask.Size()
	hostBits := bits - ones
	if hostBits == 0 {
		return []string{ip4.String()}, nil
	}
	if hostBits == 1 {
		base := binary.BigEndian.Uint32(ip4)
		out := []string{ip4.String()}
		if count > 1 {
			b := make([]byte, 4)
			binary.BigEndian.PutUint32(b, base+1)
			out = append(out, net.IP(b).String())
		}
		return out, nil
	}

	total := uint64(1) << hostBits
	// Avoid network (.0) and broadcast (.255) for /30 and larger subnets.
	usableCount := total - 2
	if usableCount == 0 {
		return []string{ip4.String()}, nil
	}
	if uint64(count) > usableCount {
		count = int(usableCount)
	}

	baseU32 := binary.BigEndian.Uint32(ip4)
	seen := make(map[string]struct{}, count)
	result := make([]string, 0, count)
	maxOffset := big.NewInt(int64(usableCount))
	maxAttempts := count * 100

	for len(result) < count && maxAttempts > 0 {
		maxAttempts--
		offsetBig, err := rand.Int(rand.Reader, maxOffset)
		if err != nil {
			return nil, err
		}
		targetU32 := baseU32 + uint32(offsetBig.Int64()+1)
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, targetU32)
		ipStr := net.IP(b).String()
		if _, exists := seen[ipStr]; !exists {
			seen[ipStr] = struct{}{}
			result = append(result, ipStr)
		}
	}
	return result, nil
}

func randomIPv6FromCIDR(ip6 net.IP, mask net.IPMask, count int) ([]string, error) {
	ones, bits := mask.Size()
	hostBits := bits - ones
	if hostBits == 0 {
		return []string{ip6.String()}, nil
	}
	if hostBits == 1 {
		baseInt := new(big.Int).SetBytes(ip6)
		out := []string{ip6.String()}
		if count > 1 {
			next := new(big.Int).Add(baseInt, big.NewInt(1))
			out = append(out, formatIPv6(next))
		}
		return out, nil
	}

	maxHost := new(big.Int).Lsh(big.NewInt(1), uint(hostBits))
	// Exclude the Subnet-Router Anycast address (all host bits 0, RFC 4291 §2.6.1).
	usableCount := new(big.Int).Sub(maxHost, big.NewInt(1))
	if usableCount.Cmp(big.NewInt(int64(count))) < 0 {
		count = int(usableCount.Int64())
	}

	baseInt := new(big.Int).SetBytes(ip6)
	seen := make(map[string]struct{}, count)
	result := make([]string, 0, count)
	maxOffset := new(big.Int).Set(usableCount)
	maxAttempts := count * 100

	for len(result) < count && maxAttempts > 0 {
		maxAttempts--
		offsetBig, err := rand.Int(rand.Reader, maxOffset)
		if err != nil {
			return nil, err
		}
		offset := new(big.Int).Add(offsetBig, big.NewInt(1))
		targetInt := new(big.Int).Add(baseInt, offset)
		ipStr := formatIPv6(targetInt)
		if _, exists := seen[ipStr]; !exists {
			seen[ipStr] = struct{}{}
			result = append(result, ipStr)
		}
	}
	return result, nil
}

func formatIPv6(val *big.Int) string {
	b := val.Bytes()
	if len(b) < 16 {
		padded := make([]byte, 16)
		copy(padded[16-len(b):], b)
		b = padded
	}
	return net.IP(b).String()
}
