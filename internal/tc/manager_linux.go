//go:build linux

package tc

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

const commandTimeout = 2 * time.Second

// ApplyClientLimit applies best-effort Linux tc limits for clients that expose
// stable IP addresses. Today that means WireGuard clients with allowedIPs. Xray
// protocols such as VLESS/VMess/Trojan multiplex many users over one port, so tc
// cannot distinguish them by email/UUID without an external packet-marking path;
// those clients are persisted but skipped here.
func ApplyClientLimit(client model.Client) error {
	if client.UploadSpeedLimit <= 0 && client.DownloadSpeedLimit <= 0 {
		return RemoveClientLimitByEmail(client.Email)
	}

	cidrs := clientCIDRs(client)
	if len(cidrs) == 0 {
		logger.Debugf("[TC] skip client %q: no classifiable client IPs", client.Email)
		return nil
	}

	iface, err := tcInterface()
	if err != nil {
		return err
	}
	minor := classMinor(client.Email)

	if err := ensureRootQdisc(iface); err != nil {
		return err
	}
	if err := clearClientFilters(iface, minor); err != nil {
		logger.Debugf("[TC] clear existing filters for %q: %v", client.Email, err)
	}

	if client.DownloadSpeedLimit > 0 {
		classID := fmt.Sprintf("1:%x", minor)
		if err := run("tc", "class", "replace", "dev", iface, "parent", "1:", "classid", classID, "htb", "rate", mbps(client.DownloadSpeedLimit), "ceil", mbps(client.DownloadSpeedLimit)); err != nil {
			return err
		}
		for _, cidr := range cidrs {
			if err := run("tc", "filter", "replace", "dev", iface, "protocol", "ip", "parent", "1:", "prio", fmt.Sprint(minor), "u32", "match", "ip", "dst", cidr, "flowid", classID); err != nil {
				return err
			}
		}
	}

	if client.UploadSpeedLimit > 0 {
		_ = run("tc", "qdisc", "add", "dev", iface, "handle", "ffff:", "ingress")
		for _, cidr := range cidrs {
			if err := run("tc", "filter", "replace", "dev", iface, "parent", "ffff:", "protocol", "ip", "prio", fmt.Sprint(minor), "u32", "match", "ip", "src", cidr, "police", "rate", mbps(client.UploadSpeedLimit), "burst", "64k", "drop", "flowid", ":1"); err != nil {
				return err
			}
		}
	}

	logger.Debugf("[TC] applied client speed limit for %q on %s", client.Email, iface)
	return nil
}

func RemoveClientLimitByEmail(email string) error {
	if strings.TrimSpace(email) == "" {
		return nil
	}
	iface, err := tcInterface()
	if err != nil {
		return nil
	}
	minor := classMinor(email)
	return clearClientFilters(iface, minor)
}

func ensureRootQdisc(iface string) error {
	if err := run("tc", "qdisc", "replace", "dev", iface, "root", "handle", "1:", "htb", "default", "1"); err != nil {
		return err
	}
	return run("tc", "class", "replace", "dev", iface, "parent", "1:", "classid", "1:1", "htb", "rate", "10000mbit", "ceil", "10000mbit")
}

func clearClientFilters(iface string, minor uint32) error {
	var errs []error
	prio := fmt.Sprint(minor)
	classID := fmt.Sprintf("1:%x", minor)
	if err := run("tc", "filter", "delete", "dev", iface, "parent", "1:", "prio", prio); err != nil {
		errs = append(errs, err)
	}
	if err := run("tc", "filter", "delete", "dev", iface, "parent", "ffff:", "prio", prio); err != nil {
		errs = append(errs, err)
	}
	if err := run("tc", "class", "delete", "dev", iface, "classid", classID); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func clientCIDRs(client model.Client) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(client.AllowedIPs))
	for _, raw := range client.AllowedIPs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if ip := net.ParseIP(raw); ip != nil {
			if ip.To4() == nil {
				continue
			}
			raw += "/32"
		} else if ip, _, err := net.ParseCIDR(raw); err != nil || ip.To4() == nil {
			continue
		}
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}
	return out
}

func tcInterface() (string, error) {
	if iface := strings.TrimSpace(os.Getenv("XUI_TC_IFACE")); iface != "" {
		return iface, nil
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		return iface.Name, nil
	}
	return "", errors.New("no active non-loopback interface found for tc")
}

func classMinor(email string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(email))))
	return h.Sum32()%60000 + 1000
}

func mbps(v float64) string {
	if v < 0.1 {
		v = 0.1
	}
	return fmt.Sprintf("%.3fmbit", v)
}

func run(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
