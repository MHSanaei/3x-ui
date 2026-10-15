package xray

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/xtls/xray-core/common/platform"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
)

func containerXrayEnv() ([]string, error) {
	binDir, err := filepath.Abs(config.GetBinFolderPath())
	if err != nil {
		return nil, err
	}
	env := os.Environ()
	// A symlink changes os.Executable's directory; keep resource lookup beside the working path.
	for _, name := range []string{platform.AssetLocation, platform.CertLocation} {
		alternate := platform.NormalizeEnvName(name)
		if _, exists := os.LookupEnv(name); exists {
			continue
		}
		if _, exists := os.LookupEnv(alternate); !exists {
			env = append(env, alternate+"="+binDir)
		}
	}
	return env, nil
}

func marshalProcessConfig(c *Config) ([]byte, error) {
	if os.Getenv("XUI_IN_DOCKER") == "true" && c != nil {
		var env map[string]string
		if len(c.Env) == 0 || json.Unmarshal(c.Env, &env) == nil {
			lookup := func(name string) string {
				for _, key := range []string{name, platform.NormalizeEnvName(name)} {
					if value, exists := env[key]; exists {
						return value
					}
					if value, exists := os.LookupEnv(key); exists {
						return value
					}
				}
				return ""
			}
			dirs := []string{lookup(platform.ConfdirLocation), lookup(platform.ConfigLocation), ".", config.GetBinFolderPath()}
			prepared := *c
			prepared.DNSConfig = resolveContainerLuaScript(c.DNSConfig, dirs)
			prepared.RouterConfig = resolveContainerLuaScript(c.RouterConfig, dirs)
			c = &prepared
		}
	}
	return json.MarshalIndent(c, "", "  ")
}

func resolveContainerLuaScript(section json_util.RawMessage, dirs []string) json_util.RawMessage {
	var cfg struct{ Script string }
	if len(section) == 0 || json.Unmarshal(section, &cfg) != nil || cfg.Script == "" || filepath.IsAbs(cfg.Script) {
		return section
	}
	// Preserve Xray's lookup order, with the working binary directory as its final fallback.
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		path, err := filepath.Abs(filepath.Join(dir, cfg.Script))
		if err != nil {
			return section
		}
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return section
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(section, &fields) != nil {
			return section
		}
		for key := range fields {
			if strings.EqualFold(key, "script") {
				delete(fields, key)
			}
		}
		fields["script"], _ = json.Marshal(path)
		data, err := json.Marshal(fields)
		if err != nil {
			return section
		}
		return data
	}
	return section
}
