package xray

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"github.com/xtls/xray-core/common/platform"
	"github.com/xtls/xray-core/infra/conf"
)

const (
	// A session ID travels in a path segment, a header, a cookie or a query value.
	maxXHTTPSessionIDLength = 1024
	maxUDPHopPorts          = 65536
	maxStreamMasks          = 16
	maxCertFileBytes        = 1 << 20
)

// CheckStreamBuildCost refuses stream settings that conf.Build(), which runs in the panel
// process, would expand without bound; settings the core cannot decode are left to it.
func CheckStreamBuildCost(raw []byte) error {
	if len(raw) == 0 {
		return nil
	}
	stream := new(conf.StreamConfig)
	if json.Unmarshal(raw, stream) != nil {
		return nil
	}
	return checkStreamBuildCost(stream)
}

func checkStreamBuildCost(stream *conf.StreamConfig) error {
	masks, hopPorts := 0, 0
	return walkStreams(stream, func(s *conf.StreamConfig, xhttp *conf.SplitHTTPConfig) error {
		if xhttp != nil {
			if length := xhttp.SessionIDLength; length.From < 0 || length.To > maxXHTTPSessionIDLength {
				return common.NewErrorf("xhttp sessionIDLength %s is outside 0-%d", length.String(), maxXHTTPSessionIDLength)
			}
		}
		if s.FinalMask == nil {
			return nil
		}
		// Every mask costs a build, and an xmc one an RSA key derivation.
		if masks += len(s.FinalMask.Tcp) + len(s.FinalMask.Udp); masks > maxStreamMasks {
			return common.NewErrorf("finalmask lists more than %d masks", maxStreamMasks)
		}
		for _, mask := range s.FinalMask.Udp {
			if !strings.EqualFold(mask.Type, "udphop") || mask.Settings == nil {
				continue
			}
			hop := new(conf.UDPHop)
			if json.Unmarshal(*mask.Settings, hop) != nil {
				continue
			}
			for _, r := range hop.RemotePorts.Range {
				// A bare number skips the core's range check, and its uint32 loop never ends on the largest.
				if r.To > 65535 {
					return common.NewErrorf("udphop remotePorts names port %d", r.To)
				}
				if r.From <= r.To {
					hopPorts += int(r.To-r.From) + 1
				}
				if hopPorts > maxUDPHopPorts {
					return common.NewErrorf("udphop remotePorts names more than %d ports", maxUDPHopPorts)
				}
			}
		}
		return nil
	})
}

// CountXMCMasks counts the xmc tcp masks of a stream, nested download streams included:
// each costs the core a fresh RSA key on every build.
func CountXMCMasks(raw []byte) int {
	stream := new(conf.StreamConfig)
	if len(raw) == 0 || json.Unmarshal(raw, stream) != nil {
		return 0
	}
	count := 0
	_ = walkStreams(stream, func(s *conf.StreamConfig, _ *conf.SplitHTTPConfig) error {
		if s.FinalMask != nil {
			for _, mask := range s.FinalMask.Tcp {
				if strings.EqualFold(mask.Type, "xmc") {
					count++
				}
			}
		}
		return nil
	})
	return count
}

// walkStreams visits stream and the download streams nested in its xhttp settings, with the
// xhttp settings the core builds: xhttpSettings over splithttpSettings, "extra" over both.
func walkStreams(stream *conf.StreamConfig, visit func(*conf.StreamConfig, *conf.SplitHTTPConfig) error) error {
	for stream != nil {
		xhttp := stream.XHTTPSettings
		if xhttp == nil {
			xhttp = stream.SplitHTTPSettings
		}
		if xhttp != nil && xhttp.Extra != nil {
			extra := new(conf.SplitHTTPConfig)
			if json.Unmarshal(xhttp.Extra, extra) != nil {
				extra = nil
			}
			xhttp = extra
		}
		if err := visit(stream, xhttp); err != nil {
			return err
		}
		stream = nil
		if xhttp != nil {
			stream = xhttp.DownloadSettings
		}
	}
	return nil
}

// CheckConfigCertFiles refuses a certificateFile or keyFile, at any depth and in any key
// case, that is not a bounded regular file: the core reads it whole inside the panel.
func CheckConfigCertFiles(raw []byte) error {
	var decoded any
	if len(raw) == 0 || json.Unmarshal(raw, &decoded) != nil {
		return nil
	}
	return checkCertFileKeys(decoded)
}

func checkCertFileKeys(v any) error {
	switch v := v.(type) {
	case map[string]any:
		for key, child := range v {
			if path, ok := child.(string); ok && path != "" && (strings.EqualFold(key, "certificateFile") || strings.EqualFold(key, "keyFile")) {
				if !filepath.IsAbs(path) {
					path = platform.GetCertLocation(path)
				}
				if err := CheckCertFile(path); err != nil {
					return err
				}
			}
			if err := checkCertFileKeys(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range v {
			if err := checkCertFileKeys(child); err != nil {
				return err
			}
		}
	}
	return nil
}

// CheckCertFile refuses a path that exists but is a device, FIFO or oversized file; a
// missing one is left to the reader, whose open fails at once.
func CheckCertFile(path string) error {
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	// procfs nodes such as /proc/kmsg report a regular file of size 0.
	if !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > maxCertFileBytes {
		return common.NewErrorf("certificate file %q must be a regular file of at most %d bytes", path, maxCertFileBytes)
	}
	return nil
}

// checkUntrustedBuildInput runs before every in-process conf.Build(): inbounds arrive from
// node pushes and outbounds from third-party subscriptions.
func checkUntrustedBuildInput(stream *conf.StreamConfig, raw []byte) error {
	if err := checkStreamBuildCost(stream); err != nil {
		return err
	}
	return CheckConfigCertFiles(raw)
}
