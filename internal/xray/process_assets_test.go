package xray

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/platform"
	"github.com/xtls/xray-core/common/platform/filesystem"
)

func runXrayAssetProcessHelper() {
	if os.Getenv("XUI_TEST_XRAY_ASSET_HELPER") == "1" {
		if len(os.Args) > 1 && os.Args[1] == "-version" {
			fmt.Println("Xray 26.10.10 fixture")
			os.Exit(0)
		}
		stopped := make(chan os.Signal, 1)
		signal.Notify(stopped, syscall.SIGTERM)
		data, err := readXrayProcessResources()
		if err != nil {
			data = fmt.Appendf(nil, "asset lookup failed: %v", err)
		}
		if writeErr := os.WriteFile(os.Getenv("XUI_TEST_XRAY_ASSET_RESULT"), data, 0o600); writeErr != nil {
			fmt.Fprintln(os.Stderr, writeErr)
			os.Exit(1)
		}
		if err != nil {
			os.Exit(1)
		}
		<-stopped
		os.Exit(0)
	}
}

func readXrayProcessResources() ([]byte, error) {
	data, err := os.ReadFile(os.Args[2])
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	var env map[string]string
	if len(c.Env) > 0 {
		if err := json.Unmarshal(c.Env, &env); err != nil {
			return nil, err
		}
	}
	for key, value := range env {
		if err := os.Setenv(key, value); err != nil {
			return nil, err
		}
	}
	data, err = os.ReadFile(platform.GetAssetLocation("geosite.dat"))
	if err != nil {
		return nil, err
	}
	cert, err := filesystem.ReadCert("certificate.pem")
	if err != nil {
		return nil, err
	}
	data = append(append(data, '\n'), cert...)
	for _, section := range [][]byte{c.DNSConfig, c.RouterConfig} {
		var cfg struct{ Script string }
		if len(section) == 0 {
			continue
		}
		if err := json.Unmarshal(section, &cfg); err != nil {
			return nil, err
		}
		if cfg.Script == "" {
			continue
		}
		path, err := platform.ResolveLuaFile(cfg.Script)
		if err != nil {
			return nil, err
		}
		script, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		data = append(append(data, '\n'), script...)
	}
	return data, nil
}

func TestSymlinkedContainerXrayFindsConfiguredResources(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test executable uses Unix signals and symlinks")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, container, override, want string
		scriptLocation                  string
	}{
		{name: "container default", container: "true", want: "geodata in bin\ncertificate in bin"},
		{name: "container explicit assets", container: "true", override: "XRAY_LOCATION_ASSET", want: "configured shared geodata\ncertificate in bin"},
		{name: "container dotted asset setting", container: "true", override: "xray.location.asset", want: "configured shared geodata\ncertificate in bin"},
		{name: "container explicit certificates", container: "true", override: "XRAY_LOCATION_CERT", want: "geodata in bin\nconfigured shared certificate"},
		{name: "container dotted certificate setting", container: "true", override: "xray.location.cert", want: "geodata in bin\nconfigured shared certificate"},
		{name: "native default", container: "false", want: "geodata in bin\ncertificate in bin"},
		{name: "container scripts beside working binary", container: "true", scriptLocation: "bin", want: "geodata in bin\ncertificate in bin\ndns in bin\nrouting in bin"},
		{name: "container scripts prefer working directory", container: "true", scriptLocation: "cwd", want: "geodata in bin\ncertificate in bin\ndns in cwd\nrouting in cwd"},
		{name: "container scripts prefer config environment", container: "true", scriptLocation: "env", want: "geodata in bin\ncertificate in bin\ndns in shared\nrouting in shared"},
		{name: "native scripts beside binary", container: "false", scriptLocation: "bin", want: "geodata in bin\ncertificate in bin\ndns in bin\nrouting in bin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			binDir := filepath.Join(root, "bin")
			imageDir := filepath.Join(root, "xray-image")
			sharedDir := filepath.Join(root, "shared-assets")
			for _, dir := range []string{binDir, imageDir, sharedDir} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, key := range []string{"XRAY_LOCATION_ASSET", "xray.location.asset", "XRAY_LOCATION_CERT", "xray.location.cert", "XRAY_LOCATION_CONFIG", "xray.location.config", "XRAY_LOCATION_CONFDIR", "xray.location.confdir"} {
				t.Setenv(key, "")
				if err := os.Unsetenv(key); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("XUI_BIN_FOLDER", binDir)
			t.Setenv("XUI_LOG_FOLDER", filepath.Join(root, "log"))
			t.Setenv("XUI_IN_DOCKER", tc.container)
			t.Setenv("XUI_TEST_XRAY_ASSET_HELPER", "1")
			resultPath := filepath.Join(root, "asset-result")
			t.Setenv("XUI_TEST_XRAY_ASSET_RESULT", resultPath)
			if tc.override != "" {
				t.Setenv(tc.override, sharedDir)
			}
			for path, content := range map[string]string{
				filepath.Join(binDir, "geosite.dat"):        "geodata in bin",
				filepath.Join(sharedDir, "geosite.dat"):     "configured shared geodata",
				filepath.Join(binDir, "certificate.pem"):    "certificate in bin",
				filepath.Join(sharedDir, "certificate.pem"): "configured shared certificate",
			} {
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			physicalPath := GetBinaryPath()
			if tc.container == "true" {
				physicalPath = filepath.Join(imageDir, GetBinaryName())
			}
			if err := os.WriteFile(physicalPath, binary, 0o755); err != nil {
				t.Fatal(err)
			}
			if tc.container == "true" {
				if err := os.Symlink(physicalPath, GetBinaryPath()); err != nil {
					t.Fatal(err)
				}
			}
			c := &Config{}
			if tc.scriptLocation != "" {
				t.Chdir(root)
				c.DNSConfig = []byte(`{"script":"dns.lua"}`)
				c.RouterConfig = []byte(`{"script":"routing.lua"}`)
				locations := map[string]string{binDir: "bin"}
				if tc.scriptLocation == "cwd" || tc.scriptLocation == "env" {
					locations[root] = "cwd"
				}
				if tc.scriptLocation == "env" {
					locations[sharedDir] = "shared"
					t.Setenv("XRAY_LOCATION_CONFDIR", binDir)
					c.Env, err = json.Marshal(map[string]string{"XRAY_LOCATION_CONFDIR": sharedDir})
					if err != nil {
						t.Fatal(err)
					}
				}
				for dir, label := range locations {
					for _, name := range []string{"dns", "routing"} {
						if err := os.WriteFile(filepath.Join(dir, name+".lua"), []byte(name+" in "+label), 0o600); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			p := NewProcess(c)
			t.Cleanup(func() { _ = p.Stop() })
			if err := p.Start(); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(3 * time.Second)
			var result []byte
			for time.Now().Before(deadline) {
				result, _ = os.ReadFile(resultPath)
				if len(result) > 0 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if string(result) != tc.want {
				t.Fatalf("Xray resource lookup = %q, want %q", result, tc.want)
			}
			if tc.scriptLocation != "" && (string(c.DNSConfig) != `{"script":"dns.lua"}` || string(c.RouterConfig) != `{"script":"routing.lua"}`) {
				t.Fatalf("starting Xray mutated its config snapshot: dns=%s routing=%s", c.DNSConfig, c.RouterConfig)
			}
		})
	}
}
