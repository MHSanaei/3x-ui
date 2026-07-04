package service

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

// PluginService owns the first public plugin contract for this fork. It does
// not load or execute plugins yet; it returns a stable schema that UI and plugin
// authors can build against while the runtime is developed.
type PluginService struct{}

const (
	PluginManifestVersion = "3x.plugin.v1"
	pluginMaxZipBytes     = 10 << 20
	pluginMaxFileBytes    = 8 << 20
	pluginManifestName    = "3x-plugin.json"
)

var (
	pluginRootDir   = filepath.Join(config.GetDBFolderPath(), "plugins")
	pluginIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{1,63}$`)
)

type PluginCatalog struct {
	ManifestVersion string             `json:"manifestVersion" example:"3x.plugin.v1"`
	Capabilities    PluginCapabilities `json:"capabilities"`
	Installed       []PluginRecord     `json:"installed"`
	Template        PluginManifest     `json:"template"`
}

type PluginCapabilities struct {
	Runtimes    []string `json:"runtimes" example:"wasm"`
	Hooks       []string `json:"hooks" example:"panel.started"`
	Permissions []string `json:"permissions" example:"inbounds:read"`
	UIZones     []string `json:"uiZones" example:"plugins.page"`
}

type PluginRecord struct {
	ID          string         `json:"id" example:"example.plugin"`
	Name        string         `json:"name" example:"Example Plugin"`
	Version     string         `json:"version" example:"0.1.0"`
	Description string         `json:"description" example:"Describe what this plugin adds to 3x-ui."`
	Author      string         `json:"author" example:"Your name or team"`
	Enabled     bool           `json:"enabled" example:"false"`
	Status      string         `json:"status" example:"installed"`
	InstalledAt string         `json:"installedAt,omitempty" example:"2026-07-04T15:00:00Z"`
	PackagePath string         `json:"packagePath,omitempty" example:"/etc/x-ui/plugins/example.plugin"`
	Manifest    PluginManifest `json:"manifest"`
}

type PluginManifest struct {
	SchemaVersion string                 `json:"schemaVersion" example:"3x.plugin.v1"`
	ID            string                 `json:"id" example:"example.plugin"`
	Name          string                 `json:"name" example:"Example Plugin"`
	Version       string                 `json:"version" example:"0.1.0"`
	Description   string                 `json:"description" example:"Describe what this plugin adds to 3x-ui."`
	Author        string                 `json:"author" example:"Your name or team"`
	Homepage      string                 `json:"homepage,omitempty" example:"https://example.com"`
	Entry         PluginEntry            `json:"entry"`
	Permissions   []PluginPermission     `json:"permissions"`
	Hooks         []PluginHook           `json:"hooks"`
	UI            []PluginUIContribution `json:"ui"`
	Config        map[string]any         `json:"config"`
}

type PluginEntry struct {
	Runtime string            `json:"runtime" example:"wasm"`
	Path    string            `json:"path,omitempty" example:"./plugin.wasm"`
	Command string            `json:"command,omitempty" example:"./plugin"`
	Args    []string          `json:"args,omitempty" example:"--verbose"`
	Env     map[string]string `json:"env,omitempty"`
}

type PluginPermission struct {
	Name   string `json:"name" example:"inbounds:read"`
	Scope  string `json:"scope" example:"panel"`
	Reason string `json:"reason" example:"Read inbound data needed by the plugin."`
}

type PluginHook struct {
	Name     string `json:"name" example:"panel.started"`
	Handler  string `json:"handler" example:"onPanelStarted"`
	Priority int    `json:"priority" example:"100"`
}

type PluginUIContribution struct {
	Zone      string `json:"zone" example:"plugins.page"`
	Label     string `json:"label" example:"Example"`
	Route     string `json:"route,omitempty" example:"/plugins/example"`
	Component string `json:"component,omitempty" example:"ExamplePanel"`
}

func (s *PluginService) GetCatalog() PluginCatalog {
	return PluginCatalog{
		ManifestVersion: PluginManifestVersion,
		Capabilities:    s.GetCapabilities(),
		Installed:       s.installed(),
		Template:        s.GetTemplate(),
	}
}

func (s *PluginService) GetCapabilities() PluginCapabilities {
	return PluginCapabilities{
		Runtimes: []string{"wasm", "process", "http"},
		Hooks: []string{
			"panel.started",
			"inbound.before_save",
			"inbound.after_save",
			"client.before_save",
			"client.after_save",
			"subscription.before_render",
			"subscription.after_render",
			"xray.before_restart",
		},
		Permissions: []string{
			"settings:read",
			"inbounds:read",
			"inbounds:write",
			"clients:read",
			"clients:write",
			"subscriptions:transform",
			"xray:restart",
			"network:egress",
		},
		UIZones: []string{
			"plugins.page",
			"dashboard.widget",
			"inbound.actions",
			"client.actions",
			"settings.section",
		},
	}
}

func (s *PluginService) GetTemplate() PluginManifest {
	return PluginManifest{
		SchemaVersion: PluginManifestVersion,
		ID:            "example.plugin",
		Name:          "Example Plugin",
		Version:       "0.1.0",
		Description:   "Describe what this plugin adds to 3x-ui.",
		Author:        "Your name or team",
		Homepage:      "https://example.com",
		Entry: PluginEntry{
			Runtime: "wasm",
			Path:    "./plugin.wasm",
		},
		Permissions: []PluginPermission{
			{Name: "inbounds:read", Scope: "panel", Reason: "Read inbound data needed by the plugin."},
		},
		Hooks: []PluginHook{
			{Name: "panel.started", Handler: "onPanelStarted", Priority: 100},
		},
		UI: []PluginUIContribution{
			{Zone: "plugins.page", Label: "Example", Route: "/plugins/example", Component: "ExamplePanel"},
		},
		Config: map[string]any{
			"enabledByDefault": false,
		},
	}
}

func (s *PluginService) InstallZip(file multipart.File, header *multipart.FileHeader) (*PluginRecord, error) {
	if header == nil {
		return nil, fmt.Errorf("missing upload metadata")
	}
	if !strings.EqualFold(filepath.Ext(header.Filename), ".zip") {
		return nil, fmt.Errorf("plugin package must be a .zip file")
	}
	if header.Size <= 0 || header.Size > pluginMaxZipBytes {
		return nil, fmt.Errorf("plugin package must be between 1 byte and %d MiB", pluginMaxZipBytes>>20)
	}

	tmp, err := os.CreateTemp("", "3x-plugin-*.zip")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(file, pluginMaxZipBytes+1))
	closeErr := tmp.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if written > pluginMaxZipBytes {
		return nil, fmt.Errorf("plugin package exceeds %d MiB", pluginMaxZipBytes>>20)
	}

	zr, err := zip.OpenReader(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("invalid zip package: %w", err)
	}
	defer zr.Close()

	manifest, err := readPluginManifest(&zr.Reader)
	if err != nil {
		return nil, err
	}
	if err := validatePluginManifest(manifest); err != nil {
		return nil, err
	}

	installDir := filepath.Join(pluginRootDir, manifest.ID)
	stagingDir := installDir + ".tmp"
	if err := os.RemoveAll(stagingDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return nil, err
	}
	if err := extractPluginZip(&zr.Reader, stagingDir); err != nil {
		_ = os.RemoveAll(stagingDir)
		return nil, err
	}
	meta := map[string]string{
		"installedAt": time.Now().UTC().Format(time.RFC3339),
		"sourceFile":  filepath.Base(header.Filename),
		"sha256":      hex.EncodeToString(hash.Sum(nil)),
	}
	if err := writeJSON(filepath.Join(stagingDir, ".3x-install.json"), meta); err != nil {
		_ = os.RemoveAll(stagingDir)
		return nil, err
	}
	if err := os.RemoveAll(installDir); err != nil {
		_ = os.RemoveAll(stagingDir)
		return nil, err
	}
	if err := os.Rename(stagingDir, installDir); err != nil {
		_ = os.RemoveAll(stagingDir)
		return nil, err
	}

	record := recordFromManifest(manifest, installDir)
	record.Status = "installed"
	record.InstalledAt = meta["installedAt"]
	return &record, nil
}

func (s *PluginService) installed() []PluginRecord {
	entries, err := os.ReadDir(pluginRootDir)
	if err != nil {
		return []PluginRecord{}
	}
	records := make([]PluginRecord, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(pluginRootDir, entry.Name())
		manifest, err := readManifestFile(filepath.Join(dir, pluginManifestName))
		if err != nil {
			manifest, err = readManifestFile(filepath.Join(dir, "plugin.json"))
		}
		if err != nil {
			continue
		}
		record := recordFromManifest(manifest, dir)
		record.Status = "installed"
		if meta, err := readInstallMeta(filepath.Join(dir, ".3x-install.json")); err == nil {
			record.InstalledAt = meta["installedAt"]
		}
		records = append(records, record)
	}
	return records
}

func readPluginManifest(zr *zip.Reader) (PluginManifest, error) {
	for _, name := range []string{pluginManifestName, "plugin.json"} {
		for _, f := range zr.File {
			if cleanZipName(f.Name) == name {
				rc, err := f.Open()
				if err != nil {
					return PluginManifest{}, err
				}
				defer rc.Close()
				var manifest PluginManifest
				if err := json.NewDecoder(io.LimitReader(rc, 1<<20)).Decode(&manifest); err != nil {
					return PluginManifest{}, fmt.Errorf("invalid %s: %w", name, err)
				}
				return manifest, nil
			}
		}
	}
	return PluginManifest{}, fmt.Errorf("missing %s manifest", pluginManifestName)
}

func validatePluginManifest(manifest PluginManifest) error {
	if manifest.SchemaVersion != PluginManifestVersion {
		return fmt.Errorf("unsupported plugin schema version %q", manifest.SchemaVersion)
	}
	if !pluginIDPattern.MatchString(manifest.ID) {
		return fmt.Errorf("invalid plugin id %q", manifest.ID)
	}
	if strings.TrimSpace(manifest.Name) == "" || strings.TrimSpace(manifest.Version) == "" {
		return fmt.Errorf("plugin name and version are required")
	}
	switch manifest.Entry.Runtime {
	case "wasm", "process", "http":
	default:
		return fmt.Errorf("unsupported plugin runtime %q", manifest.Entry.Runtime)
	}
	return nil
}

func extractPluginZip(zr *zip.Reader, dest string) error {
	for _, f := range zr.File {
		name := cleanZipName(f.Name)
		if name == "" {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if !strings.HasPrefix(target, dest+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe zip path %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if f.UncompressedSize64 > pluginMaxFileBytes {
			return fmt.Errorf("plugin file %q exceeds %d MiB", name, pluginMaxFileBytes>>20)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		if err := writeFileFromReader(target, rc); err != nil {
			rc.Close()
			return err
		}
		rc.Close()
	}
	return nil
}

func cleanZipName(name string) string {
	name = strings.TrimLeft(filepath.ToSlash(name), "/")
	if name == "." || strings.Contains(name, "..") {
		return ""
	}
	return name
}

func writeFileFromReader(target string, r io.Reader) error {
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	written, err := io.Copy(out, io.LimitReader(r, pluginMaxFileBytes+1))
	if err != nil {
		return err
	}
	if written > pluginMaxFileBytes {
		return fmt.Errorf("plugin file exceeds %d MiB", pluginMaxFileBytes>>20)
	}
	return nil
}

func readManifestFile(path string) (PluginManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return PluginManifest{}, err
	}
	defer file.Close()
	var manifest PluginManifest
	err = json.NewDecoder(file).Decode(&manifest)
	return manifest, err
}

func readInstallMeta(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	meta := map[string]string{}
	err = json.NewDecoder(file).Decode(&meta)
	return meta, err
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func recordFromManifest(manifest PluginManifest, dir string) PluginRecord {
	return PluginRecord{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Version:     manifest.Version,
		Description: manifest.Description,
		Author:      manifest.Author,
		Enabled:     false,
		Status:      "installed",
		PackagePath: dir,
		Manifest:    manifest,
	}
}
