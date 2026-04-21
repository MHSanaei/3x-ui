package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
)

const (
	customGeoTypeGeosite  = "geosite"
	customGeoTypeGeoip    = "geoip"
	minDatBytes           = 64
	customGeoProbeTimeout = 12 * time.Second
)

var (
	customGeoAliasPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)
	reservedCustomAliases = map[string]struct{}{
		"geoip": {}, "geosite": {},
		"geoip_ir": {}, "geosite_ir": {},
		"geoip_ru": {}, "geosite_ru": {},
	}
	ErrCustomGeoInvalidType    = errors.New("custom_geo_invalid_type")
	ErrCustomGeoAliasRequired  = errors.New("custom_geo_alias_required")
	ErrCustomGeoAliasPattern   = errors.New("custom_geo_alias_pattern")
	ErrCustomGeoAliasReserved  = errors.New("custom_geo_alias_reserved")
	ErrCustomGeoURLRequired    = errors.New("custom_geo_url_required")
	ErrCustomGeoInvalidURL     = errors.New("custom_geo_invalid_url")
	ErrCustomGeoURLScheme      = errors.New("custom_geo_url_scheme")
	ErrCustomGeoURLHost        = errors.New("custom_geo_url_host")
	ErrCustomGeoDuplicateAlias = errors.New("custom_geo_duplicate_alias")
	ErrCustomGeoNotFound       = errors.New("custom_geo_not_found")
	ErrCustomGeoDownload       = errors.New("custom_geo_download")
	ErrCustomGeoSSRFBlocked    = errors.New("custom_geo_ssrf_blocked")
	ErrCustomGeoPathTraversal  = errors.New("custom_geo_path_traversal")
)

type CustomGeoUpdateAllItem struct {
	Id       int    `json:"id"`
	Alias    string `json:"alias"`
	FileName string `json:"fileName"`
}

type CustomGeoUpdateAllFailure struct {
	Id       int    `json:"id"`
	Alias    string `json:"alias"`
	FileName string `json:"fileName"`
	Err      string `json:"error"`
}

type CustomGeoUpdateAllResult struct {
	Succeeded []CustomGeoUpdateAllItem    `json:"succeeded"`
	Failed    []CustomGeoUpdateAllFailure `json:"failed"`
}

type CustomGeoService struct {
	serverService    *ServerService
	updateAllGetAll  func() ([]model.CustomGeoResource, error)
	updateAllApply   func(id int, onStartup bool) (string, error)
	updateAllRestart func() error
}

func NewCustomGeoService() *CustomGeoService {
	s := &CustomGeoService{
		serverService: &ServerService{},
	}
	s.updateAllGetAll = s.GetAll
	s.updateAllApply = s.applyDownloadAndPersist
	s.updateAllRestart = func() error { return s.serverService.RestartXrayService() }
	return s
}

func NormalizeAliasKey(alias string) string {
	return strings.ToLower(strings.ReplaceAll(alias, "-", "_"))
}

func (s *CustomGeoService) fileNameFor(typ, alias string) string {
	if typ == customGeoTypeGeoip {
		return fmt.Sprintf("geoip_%s.dat", alias)
	}
	return fmt.Sprintf("geosite_%s.dat", alias)
}

func (s *CustomGeoService) validateType(typ string) error {
	if typ != customGeoTypeGeosite && typ != customGeoTypeGeoip {
		return ErrCustomGeoInvalidType
	}
	return nil
}

func (s *CustomGeoService) validateAlias(alias string) error {
	if alias == "" {
		return ErrCustomGeoAliasRequired
	}
	if !customGeoAliasPattern.MatchString(alias) {
		return ErrCustomGeoAliasPattern
	}
	if _, ok := reservedCustomAliases[NormalizeAliasKey(alias)]; ok {
		return ErrCustomGeoAliasReserved
	}
	return nil
}

func (s *CustomGeoService) sanitizeURL(raw string) (string, error) {
	if raw == "" {
		return "", ErrCustomGeoURLRequired
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", ErrCustomGeoInvalidURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrCustomGeoURLScheme
	}
	if u.Host == "" {
		return "", ErrCustomGeoURLHost
	}
	if err := checkSSRF(context.Background(), u.Hostname()); err != nil {
		return "", err
	}
	// Reconstruct URL from parsed components to break taint propagation.
	clean := &url.URL{
		Scheme:   u.Scheme,
		Host:     u.Host,
		Path:     u.Path,
		RawPath:  u.RawPath,
		RawQuery: u.RawQuery,
		Fragment: u.Fragment,
	}
	return clean.String(), nil
}

func localDatFileNeedsRepair(path string) bool {
	safePath, err := sanitizeDestPath(path)
	if err != nil {
		return true
	}
	fi, err := os.Stat(safePath)
	if err != nil {
		return true
	}
	if fi.IsDir() {
		return true
	}
	return fi.Size() < int64(minDatBytes)
}

func CustomGeoLocalFileNeedsRepair(path string) bool {
	return localDatFileNeedsRepair(path)
}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// checkSSRFDefault validates that the given host does not resolve to a private/internal IP.
// It is context-aware so that dial context cancellation/deadlines are respected during DNS resolution.
func checkSSRFDefault(ctx context.Context, hostname string) error {
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, hostname)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve host %s", ErrCustomGeoSSRFBlocked, hostname)
	}
	for _, ipAddr := range ips {
		if isBlockedIP(ipAddr.IP) {
			return fmt.Errorf("%w: %s resolves to blocked address %s", ErrCustomGeoSSRFBlocked, hostname, ipAddr.IP)
		}
	}
	return nil
}

// checkSSRF is the active SSRF guard. Override in tests to allow localhost test servers.
var checkSSRF = checkSSRFDefault

func ssrfSafeTransport() http.RoundTripper {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	cloned := base.Clone()
	cloned.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCustomGeoSSRFBlocked, err)
		}
		if err := checkSSRF(ctx, host); err != nil {
			return nil, err
		}
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, addr)
	}
	return cloned
}

func probeCustomGeoURLWithGET(rawURL string) error {
	sanitizedURL, err := (&CustomGeoService{}).sanitizeURL(rawURL)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: customGeoProbeTimeout, Transport: ssrfSafeTransport()}
	req, err := http.NewRequest(http.MethodGet, sanitizedURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", "bytes=0-0")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 256))
	switch resp.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
		return nil
	default:
		return fmt.Errorf("get range status %d", resp.StatusCode)
	}
}

func probeCustomGeoURL(rawURL string) error {
	sanitizedURL, err := (&CustomGeoService{}).sanitizeURL(rawURL)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: customGeoProbeTimeout, Transport: ssrfSafeTransport()}
	req, err := http.NewRequest(http.MethodHead, sanitizedURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	sc := resp.StatusCode
	if sc >= 200 && sc < 300 {
		return nil
	}
	if sc == http.StatusMethodNotAllowed || sc == http.StatusNotImplemented {
		return probeCustomGeoURLWithGET(rawURL)
	}
	return fmt.Errorf("head status %d", sc)
}

func (s *CustomGeoService) EnsureOnStartup() {
	list, err := s.GetAll()
	if err != nil {
		logger.Warning("custom geo startup: load list:", err)
		return
	}
	n := len(list)
	if n == 0 {
		logger.Info("custom geo startup: no custom geofiles configured")
		return
	}
	logger.Infof("custom geo startup: checking %d custom geofile(s)", n)
	for i := range list {
		r := &list[i]
		sanitizedURL, err := s.sanitizeURL(r.Url)
		if err != nil {
			logger.Warningf("custom geo startup id=%d: invalid url: %v", r.Id, err)
			continue
		}
		r.Url = sanitizedURL
		s.syncLocalPath(r)
		localPath := r.LocalPath
		if !localDatFileNeedsRepair(localPath) {
			logger.Infof("custom geo startup id=%d alias=%s path=%s: present", r.Id, r.Alias, localPath)
			continue
		}
		logger.Infof("custom geo startup id=%d alias=%s path=%s: missing or needs repair, probing source", r.Id, r.Alias, localPath)
		if err := probeCustomGeoURL(r.Url); err != nil {
			logger.Warningf("custom geo startup id=%d alias=%s url=%s: probe: %v (attempting download anyway)", r.Id, r.Alias, r.Url, err)
		}
		_, _ = s.applyDownloadAndPersist(r.Id, true)
	}
}

func (s *CustomGeoService) downloadToPath(resourceURL, destPath string, lastModifiedHeader string) (skipped bool, newLastModified string, err error) {
	safeDestPath, err := sanitizeDestPath(destPath)
	if err != nil {
		return false, "", fmt.Errorf("%w: %v", ErrCustomGeoDownload, err)
	}

	skipped, lm, err := s.downloadToPathOnce(resourceURL, safeDestPath, lastModifiedHeader, false)
	if err != nil {
		return false, "", err
	}
	if skipped {
		if _, statErr := os.Stat(safeDestPath); statErr == nil && !localDatFileNeedsRepair(safeDestPath) {
			return true, lm, nil
		}
		return s.downloadToPathOnce(resourceURL, safeDestPath, lastModifiedHeader, true)
	}
	return false, lm, nil
}

// sanitizeDestPath ensures destPath is inside the bin folder, preventing path traversal.
// It resolves symlinks to prevent symlink-based escapes.
// Returns the cleaned absolute path that is safe to use in file operations.
func sanitizeDestPath(destPath string) (string, error) {
	baseDirAbs, err := filepath.Abs(config.GetBinFolderPath())
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrCustomGeoPathTraversal, err)
	}
	// Resolve symlinks in base directory to get the real path.
	if resolved, evalErr := filepath.EvalSymlinks(baseDirAbs); evalErr == nil {
		baseDirAbs = resolved
	}
	destPathAbs, err := filepath.Abs(destPath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrCustomGeoPathTraversal, err)
	}
	// Resolve symlinks for the parent directory of the destination path.
	destDir := filepath.Dir(destPathAbs)
	if resolved, evalErr := filepath.EvalSymlinks(destDir); evalErr == nil {
		destPathAbs = filepath.Join(resolved, filepath.Base(destPathAbs))
	}
	// Verify the resolved path is within the safe base directory using prefix check.
	safeDirPrefix := baseDirAbs + string(filepath.Separator)
	if !strings.HasPrefix(destPathAbs, safeDirPrefix) {
		return "", ErrCustomGeoPathTraversal
	}
	return destPathAbs, nil
}

func (s *CustomGeoService) downloadToPathOnce(resourceURL, destPath string, lastModifiedHeader string, forceFull bool) (skipped bool, newLastModified string, err error) {
	safeDestPath, err := sanitizeDestPath(destPath)
	if err != nil {
		return false, "", fmt.Errorf("%w: %v", ErrCustomGeoDownload, err)
	}
	sanitizedURL, err := s.sanitizeURL(resourceURL)
	if err != nil {
		return false, "", fmt.Errorf("%w: %v", ErrCustomGeoDownload, err)
	}

	var req *http.Request
	req, err = http.NewRequest(http.MethodGet, sanitizedURL, nil)
	if err != nil {
		return false, "", fmt.Errorf("%w: %v", ErrCustomGeoDownload, err)
	}

	if !forceFull {
		if fi, statErr := os.Stat(safeDestPath); statErr == nil && !localDatFileNeedsRepair(safeDestPath) {
			if !fi.ModTime().IsZero() {
				req.Header.Set("If-Modified-Since", fi.ModTime().UTC().Format(http.TimeFormat))
			} else if lastModifiedHeader != "" {
				if t, perr := time.Parse(http.TimeFormat, lastModifiedHeader); perr == nil {
					req.Header.Set("If-Modified-Since", t.UTC().Format(http.TimeFormat))
				}
			}
		}
	}

	client := &http.Client{Timeout: 10 * time.Minute, Transport: ssrfSafeTransport()}
	// lgtm[go/request-forgery]
	resp, err := client.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("%w: %v", ErrCustomGeoDownload, err)
	}
	defer resp.Body.Close()

	var serverModTime time.Time
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		if parsed, perr := time.Parse(http.TimeFormat, lm); perr == nil {
			serverModTime = parsed
			newLastModified = lm
		}
	}

	updateModTime := func() {
		if !serverModTime.IsZero() {
			_ = os.Chtimes(safeDestPath, serverModTime, serverModTime)
		}
	}

	if resp.StatusCode == http.StatusNotModified {
		if forceFull {
			return false, "", fmt.Errorf("%w: unexpected 304 on unconditional get", ErrCustomGeoDownload)
		}
		updateModTime()
		return true, newLastModified, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("%w: unexpected status %d", ErrCustomGeoDownload, resp.StatusCode)
	}

	binDir := filepath.Dir(safeDestPath)
	if err = os.MkdirAll(binDir, 0o755); err != nil {
		return false, "", fmt.Errorf("%w: %v", ErrCustomGeoDownload, err)
	}

	safeTmpPath, err := sanitizeDestPath(safeDestPath + ".tmp")
	if err != nil {
		return false, "", fmt.Errorf("%w: %v", ErrCustomGeoDownload, err)
	}
	out, err := os.Create(safeTmpPath)
	if err != nil {
		return false, "", fmt.Errorf("%w: %v", ErrCustomGeoDownload, err)
	}
	n, err := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if err != nil {
		_ = os.Remove(safeTmpPath)
		return false, "", fmt.Errorf("%w: %v", ErrCustomGeoDownload, err)
	}
	if closeErr != nil {
		_ = os.Remove(safeTmpPath)
		return false, "", fmt.Errorf("%w: %v", ErrCustomGeoDownload, closeErr)
	}
	if n < minDatBytes {
		_ = os.Remove(safeTmpPath)
		return false, "", fmt.Errorf("%w: file too small", ErrCustomGeoDownload)
	}

	if err = os.Rename(safeTmpPath, safeDestPath); err != nil {
		_ = os.Remove(safeTmpPath)
		return false, "", fmt.Errorf("%w: %v", ErrCustomGeoDownload, err)
	}

	updateModTime()
	if newLastModified == "" && resp.Header.Get("Last-Modified") != "" {
		newLastModified = resp.Header.Get("Last-Modified")
	}
	return false, newLastModified, nil
}

func (s *CustomGeoService) resolveDestPath(r *model.CustomGeoResource) string {
	if r.LocalPath != "" {
		return r.LocalPath
	}
	return filepath.Join(config.GetBinFolderPath(), s.fileNameFor(r.Type, r.Alias))
}

func (s *CustomGeoService) syncLocalPath(r *model.CustomGeoResource) {
	p := filepath.Join(config.GetBinFolderPath(), s.fileNameFor(r.Type, r.Alias))
	r.LocalPath = p
}

func (s *CustomGeoService) syncAndSanitizeLocalPath(r *model.CustomGeoResource) error {
	s.syncLocalPath(r)
	safePath, err := sanitizeDestPath(r.LocalPath)
	if err != nil {
		return err
	}
	r.LocalPath = safePath
	return nil
}

func removeSafePathIfExists(path string) error {
	safePath, err := sanitizeDestPath(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(safePath); err == nil {
		if err := os.Remove(safePath); err != nil {
			return err
		}
	}
	return nil
}

func (s *CustomGeoService) Create(r *model.CustomGeoResource) error {
	if err := s.validateType(r.Type); err != nil {
		return err
	}
	if err := s.validateAlias(r.Alias); err != nil {
		return err
	}
	sanitizedURL, err := s.sanitizeURL(r.Url)
	if err != nil {
		return err
	}
	r.Url = sanitizedURL
	var existing int64
	database.GetDB().Model(&model.CustomGeoResource{}).
		Where("geo_type = ? AND alias = ?", r.Type, r.Alias).Count(&existing)
	if existing > 0 {
		return ErrCustomGeoDuplicateAlias
	}
	if err := s.syncAndSanitizeLocalPath(r); err != nil {
		return err
	}
	skipped, lm, err := s.downloadToPath(r.Url, r.LocalPath, r.LastModified)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	r.LastUpdatedAt = now
	r.LastModified = lm
	if err = database.GetDB().Create(r).Error; err != nil {
		_ = removeSafePathIfExists(r.LocalPath)
		return err
	}
	logger.Infof("custom geo created id=%d type=%s alias=%s skipped=%v", r.Id, r.Type, r.Alias, skipped)
	if err = s.serverService.RestartXrayService(); err != nil {
		logger.Warning("custom geo create: restart xray:", err)
	}
	return nil
}

func (s *CustomGeoService) Update(id int, r *model.CustomGeoResource) error {
	var cur model.CustomGeoResource
	if err := database.GetDB().First(&cur, id).Error; err != nil {
		if database.IsNotFound(err) {
			return ErrCustomGeoNotFound
		}
		return err
	}
	if err := s.validateType(r.Type); err != nil {
		return err
	}
	if err := s.validateAlias(r.Alias); err != nil {
		return err
	}
	sanitizedURL, err := s.sanitizeURL(r.Url)
	if err != nil {
		return err
	}
	r.Url = sanitizedURL
	if cur.Type != r.Type || cur.Alias != r.Alias {
		var cnt int64
		database.GetDB().Model(&model.CustomGeoResource{}).
			Where("geo_type = ? AND alias = ? AND id <> ?", r.Type, r.Alias, id).
			Count(&cnt)
		if cnt > 0 {
			return ErrCustomGeoDuplicateAlias
		}
	}
	oldPath := s.resolveDestPath(&cur)
	r.Id = id
	if err := s.syncAndSanitizeLocalPath(r); err != nil {
		return err
	}
	if oldPath != r.LocalPath && oldPath != "" {
		if err := removeSafePathIfExists(oldPath); err != nil && !errors.Is(err, ErrCustomGeoPathTraversal) {
			logger.Warningf("custom geo remove old path %s: %v", oldPath, err)
		}
	}
	_, lm, err := s.downloadToPath(r.Url, r.LocalPath, cur.LastModified)
	if err != nil {
		return err
	}
	r.LastUpdatedAt = time.Now().Unix()
	r.LastModified = lm
	err = database.GetDB().Model(&model.CustomGeoResource{}).Where("id = ?", id).Updates(map[string]any{
		"geo_type":        r.Type,
		"alias":           r.Alias,
		"url":             r.Url,
		"local_path":      r.LocalPath,
		"last_updated_at": r.LastUpdatedAt,
		"last_modified":   r.LastModified,
	}).Error
	if err != nil {
		return err
	}
	logger.Infof("custom geo updated id=%d", id)
	if err = s.serverService.RestartXrayService(); err != nil {
		logger.Warning("custom geo update: restart xray:", err)
	}
	return nil
}

func (s *CustomGeoService) Delete(id int) (displayName string, err error) {
	var r model.CustomGeoResource
	if err := database.GetDB().First(&r, id).Error; err != nil {
		if database.IsNotFound(err) {
			return "", ErrCustomGeoNotFound
		}
		return "", err
	}
	displayName = s.fileNameFor(r.Type, r.Alias)
	p := s.resolveDestPath(&r)
	if _, err := sanitizeDestPath(p); err != nil {
		return displayName, err
	}
	if err := database.GetDB().Delete(&model.CustomGeoResource{}, id).Error; err != nil {
		return displayName, err
	}
	if p != "" {
		if err := removeSafePathIfExists(p); err != nil {
			logger.Warningf("custom geo delete file %s: %v", p, err)
		}
	}
	logger.Infof("custom geo deleted id=%d", id)
	if err := s.serverService.RestartXrayService(); err != nil {
		logger.Warning("custom geo delete: restart xray:", err)
	}
	return displayName, nil
}

func (s *CustomGeoService) GetAll() ([]model.CustomGeoResource, error) {
	var list []model.CustomGeoResource
	err := database.GetDB().Order("id asc").Find(&list).Error
	return list, err
}

func (s *CustomGeoService) applyDownloadAndPersist(id int, onStartup bool) (displayName string, err error) {
	var r model.CustomGeoResource
	if err := database.GetDB().First(&r, id).Error; err != nil {
		if database.IsNotFound(err) {
			return "", ErrCustomGeoNotFound
		}
		return "", err
	}
	displayName = s.fileNameFor(r.Type, r.Alias)
	if err := s.syncAndSanitizeLocalPath(&r); err != nil {
		return displayName, err
	}
	sanitizedURL, sanitizeErr := s.sanitizeURL(r.Url)
	if sanitizeErr != nil {
		return displayName, sanitizeErr
	}
	skipped, lm, err := s.downloadToPath(sanitizedURL, r.LocalPath, r.LastModified)
	if err != nil {
		if onStartup {
			logger.Warningf("custom geo startup download id=%d: %v", id, err)
		} else {
			logger.Warningf("custom geo manual update id=%d: %v", id, err)
		}
		return displayName, err
	}
	now := time.Now().Unix()
	updates := map[string]any{
		"last_modified":   lm,
		"local_path":      r.LocalPath,
		"last_updated_at": now,
	}
	if err = database.GetDB().Model(&model.CustomGeoResource{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		if onStartup {
			logger.Warningf("custom geo startup id=%d: persist metadata: %v", id, err)
		} else {
			logger.Warningf("custom geo manual update id=%d: persist metadata: %v", id, err)
		}
		return displayName, err
	}
	if skipped {
		if onStartup {
			logger.Infof("custom geo startup download skipped (not modified) id=%d", id)
		} else {
			logger.Infof("custom geo manual update skipped (not modified) id=%d", id)
		}
	} else {
		if onStartup {
			logger.Infof("custom geo startup download ok id=%d", id)
		} else {
			logger.Infof("custom geo manual update ok id=%d", id)
		}
	}
	return displayName, nil
}

func (s *CustomGeoService) TriggerUpdate(id int) (string, error) {
	displayName, err := s.applyDownloadAndPersist(id, false)
	if err != nil {
		return displayName, err
	}
	if err = s.serverService.RestartXrayService(); err != nil {
		logger.Warning("custom geo manual update: restart xray:", err)
	}
	return displayName, nil
}

func (s *CustomGeoService) TriggerUpdateAll() (*CustomGeoUpdateAllResult, error) {
	var list []model.CustomGeoResource
	var err error
	if s.updateAllGetAll != nil {
		list, err = s.updateAllGetAll()
	} else {
		list, err = s.GetAll()
	}
	if err != nil {
		return nil, err
	}
	res := &CustomGeoUpdateAllResult{}
	if len(list) == 0 {
		return res, nil
	}
	for _, r := range list {
		var name string
		var applyErr error
		if s.updateAllApply != nil {
			name, applyErr = s.updateAllApply(r.Id, false)
		} else {
			name, applyErr = s.applyDownloadAndPersist(r.Id, false)
		}
		if applyErr != nil {
			res.Failed = append(res.Failed, CustomGeoUpdateAllFailure{
				Id: r.Id, Alias: r.Alias, FileName: name, Err: applyErr.Error(),
			})
			continue
		}
		res.Succeeded = append(res.Succeeded, CustomGeoUpdateAllItem{
			Id: r.Id, Alias: r.Alias, FileName: name,
		})
	}
	if len(res.Succeeded) > 0 {
		var restartErr error
		if s.updateAllRestart != nil {
			restartErr = s.updateAllRestart()
		} else {
			restartErr = s.serverService.RestartXrayService()
		}
		if restartErr != nil {
			logger.Warning("custom geo update all: restart xray:", restartErr)
		}
	}
	return res, nil
}

type CustomGeoAliasItem struct {
	Alias      string `json:"alias"`
	Type       string `json:"type"`
	FileName   string `json:"fileName"`
	ExtExample string `json:"extExample"`
}

type CustomGeoAliasesResponse struct {
	Geosite []CustomGeoAliasItem `json:"geosite"`
	Geoip   []CustomGeoAliasItem `json:"geoip"`
}

func (s *CustomGeoService) GetAliasesForUI() (CustomGeoAliasesResponse, error) {
	list, err := s.GetAll()
	if err != nil {
		logger.Warning("custom geo GetAliasesForUI:", err)
		return CustomGeoAliasesResponse{}, err
	}
	var out CustomGeoAliasesResponse
	for _, r := range list {
		fn := s.fileNameFor(r.Type, r.Alias)
		ex := fmt.Sprintf("ext:%s:tag", fn)
		item := CustomGeoAliasItem{
			Alias:      r.Alias,
			Type:       r.Type,
			FileName:   fn,
			ExtExample: ex,
		}
		if r.Type == customGeoTypeGeoip {
			out.Geoip = append(out.Geoip, item)
		} else {
			out.Geosite = append(out.Geosite, item)
		}
	}
	return out, nil
}
