package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"regexp"

	"x-ui/config"
	"x-ui/database"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/util/sys"
	"x-ui/xray"
	"x-ui/web/global"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

type ProcessState string

const (
	Running ProcessState = "running"
	Stop    ProcessState = "stop"
	Error   ProcessState = "error"
)

type Status struct {
	T           time.Time `json:"-"`
	Cpu         float64   `json:"cpu"`
	CpuCores    int       `json:"cpuCores"`
	LogicalPro  int       `json:"logicalPro"`
	CpuSpeedMhz float64   `json:"cpuSpeedMhz"`
	Mem         struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"mem"`
	Swap struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"swap"`
	Disk struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"disk"`
	Xray struct {
		State    ProcessState `json:"state"`
		ErrorMsg string       `json:"errorMsg"`
		Version  string       `json:"version"`
	} `json:"xray"`
	XUI struct {
		LatestVersion string `json:"latestVersion"`
	} `json:"xui"`
	Uptime   uint64    `json:"uptime"`
	Loads    []float64 `json:"loads"`
	TcpCount int       `json:"tcpCount"`
	UdpCount int       `json:"udpCount"`
	NetIO    struct {
		Up   uint64 `json:"up"`
		Down uint64 `json:"down"`
	} `json:"netIO"`
	NetTraffic struct {
		Sent uint64 `json:"sent"`
		Recv uint64 `json:"recv"`
	} `json:"netTraffic"`
	PublicIP struct {
		IPv4 string `json:"ipv4"`
		IPv6 string `json:"ipv6"`
	} `json:"publicIP"`
	AppStats struct {
		Threads uint32 `json:"threads"`
		Mem     uint64 `json:"mem"`
		Uptime  uint64 `json:"uptime"`
	} `json:"appStats"`
}

type Release struct {
	TagName string `json:"tag_name"`
}

type ServerService struct {
	xrayService    XrayService
	inboundService InboundService
}

func extractValue(body string, key string) string {
    keystr := "\"" + key + "\":[^,;\\]}]*"
    r, _ := regexp.Compile(keystr)
    match := r.FindString(body)
    keyValMatch := strings.Split(match, ":")
    return strings.TrimSpace(strings.ReplaceAll(keyValMatch[1], "\"", ""))
}

func getPublicIP(url string) string {
	var host string
	host = os.Getenv("XUI_SERVER_IP")
	if host != "" && !strings.ContainsAny(url, "6") {
		return host
	}

	resp, err := http.Get(url)
	if err != nil {
		return "N/A"
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "N/A"
	}

	ipString := string(ip)
	if ipString == "" {
		return "N/A"
	}

	return ipString
}

func getXuiLatestVersion() string {
	cache := global.GetCache().Memory()
	if data, found := cache.Get("xui_latest_tag_name"); found {
		if tag, ok := data.(string); ok {
			return string(tag)
		} else {
			return ""
		}
	} else {
		url := "https://api.github.com/repos/MHSanaei/3x-ui/releases/latest"
		
		resp, err := http.Get(url)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()
		
		json, err := io.ReadAll(resp.Body)
		if err != nil {
			return ""
		}

		tag := extractValue(string(json), "tag_name")
		cache.Set("xui_latest_tag_name", tag, 60*time.Minute)
		return tag
	}
}

func (s *ServerService) GetStatus(lastStatus *Status) *Status {
	cache := global.GetCache().Memory()

	now := time.Now()
	status := &Status{
		T: now,
	}

	percents, err := cpu.Percent(0, false)
	if err != nil {
		logger.Warning("get cpu percent failed:", err)
	} else {
		status.Cpu = percents[0]
	}

	status.CpuCores, err = cpu.Counts(false)
	if err != nil {
		logger.Warning("get cpu cores count failed:", err)
	}

	status.LogicalPro = runtime.NumCPU()
	if p != nil && p.IsRunning() {
		status.AppStats.Uptime = p.GetUptime()
	} else {
		status.AppStats.Uptime = 0
	}

	cpuInfos, err := cpu.Info()
	if err != nil {
		logger.Warning("get cpu info failed:", err)
	} else if len(cpuInfos) > 0 {
		cpuInfo := cpuInfos[0]
		status.CpuSpeedMhz = cpuInfo.Mhz // setting CPU speed in MHz
	} else {
		logger.Warning("could not find cpu info")
	}

	upTime, err := host.Uptime()
	if err != nil {
		logger.Warning("get uptime failed:", err)
	} else {
		status.Uptime = upTime
	}

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		logger.Warning("get virtual memory failed:", err)
	} else {
		status.Mem.Current = memInfo.Used
		status.Mem.Total = memInfo.Total
	}

	swapInfo, err := mem.SwapMemory()
	if err != nil {
		logger.Warning("get swap memory failed:", err)
	} else {
		status.Swap.Current = swapInfo.Used
		status.Swap.Total = swapInfo.Total
	}

	distInfo, err := disk.Usage("/")
	if err != nil {
		logger.Warning("get dist usage failed:", err)
	} else {
		status.Disk.Current = distInfo.Used
		status.Disk.Total = distInfo.Total
	}

	avgState, err := load.Avg()
	if err != nil {
		logger.Warning("get load avg failed:", err)
	} else {
		status.Loads = []float64{avgState.Load1, avgState.Load5, avgState.Load15}
	}

	ioStats, err := net.IOCounters(false)
	if err != nil {
		logger.Warning("get io counters failed:", err)
	} else if len(ioStats) > 0 {
		ioStat := ioStats[0]
		status.NetTraffic.Sent = ioStat.BytesSent
		status.NetTraffic.Recv = ioStat.BytesRecv

		if lastStatus != nil {
			duration := now.Sub(lastStatus.T)
			seconds := float64(duration) / float64(time.Second)
			up := uint64(float64(status.NetTraffic.Sent-lastStatus.NetTraffic.Sent) / seconds)
			down := uint64(float64(status.NetTraffic.Recv-lastStatus.NetTraffic.Recv) / seconds)
			status.NetIO.Up = up
			status.NetIO.Down = down
		}
	} else {
		logger.Warning("can not find io counters")
	}

	status.TcpCount, err = sys.GetTCPCount()
	if err != nil {
		logger.Warning("get tcp connections failed:", err)
	}

	status.UdpCount, err = sys.GetUDPCount()
	if err != nil {
		logger.Warning("get udp connections failed:", err)
	}

	if data, found := cache.Get("xui_public_ipv4"); found {
		if ipv4, ok := data.(string); ok {
			status.PublicIP.IPv4 = string(ipv4)
		} else {
			status.PublicIP.IPv4 = "N/A"
		}
	} else {
		status.PublicIP.IPv4 = getPublicIP("https://api.ipify.org")
		cache.Set("xui_public_ipv4", status.PublicIP.IPv4, 720*time.Hour)
	}

	if data, found := cache.Get("xui_public_ipv6"); found {
		if ipv6, ok := data.(string); ok {
			status.PublicIP.IPv6 = string(ipv6)
		} else {
			status.PublicIP.IPv6 = "N/A"
		}
	} else {
		status.PublicIP.IPv6 = getPublicIP("https://api6.ipify.org")
		cache.Set("xui_public_ipv6", status.PublicIP.IPv6, 720*time.Hour)
	}

	if s.xrayService.IsXrayRunning() {
		status.Xray.State = Running
		status.Xray.ErrorMsg = ""
	} else {
		err := s.xrayService.GetXrayErr()
		if err != nil {
			status.Xray.State = Error
		} else {
			status.Xray.State = Stop
		}
		status.Xray.ErrorMsg = s.xrayService.GetXrayResult()
	}
	status.Xray.Version = s.xrayService.GetXrayVersion()

	status.XUI.LatestVersion = getXuiLatestVersion()

	var rtm runtime.MemStats
	runtime.ReadMemStats(&rtm)

	status.AppStats.Mem = rtm.Sys
	status.AppStats.Threads = uint32(runtime.NumGoroutine())
	if p != nil && p.IsRunning() {
		status.AppStats.Uptime = p.GetUptime()
	} else {
		status.AppStats.Uptime = 0
	}

	return status
}

func (s *ServerService) GetXrayVersions() ([]string, error) {
	const (
		XrayURL    = "https://api.github.com/repos/XTLS/Xray-core/releases"
		bufferSize = 8192
	)

	resp, err := http.Get(XrayURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buffer := bytes.NewBuffer(make([]byte, bufferSize))
	buffer.Reset()
	if _, err := buffer.ReadFrom(resp.Body); err != nil {
		return nil, err
	}

	var releases []Release
	if err := json.Unmarshal(buffer.Bytes(), &releases); err != nil {
		return nil, err
	}

	var versions []string
	for _, release := range releases {
		tagVersion := strings.TrimPrefix(release.TagName, "v")
		tagParts := strings.Split(tagVersion, ".")
		if len(tagParts) != 3 {
			continue
		}

		major, err1 := strconv.Atoi(tagParts[0])
		minor, err2 := strconv.Atoi(tagParts[1])
		patch, err3 := strconv.Atoi(tagParts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}

		if (major == 1 && minor == 8 && patch == 24) ||
			(major >= 25) {
			versions = append(versions, release.TagName)
		}
	}
	return versions, nil
}

func (s *ServerService) StopXrayService() (string error) {
	err := s.xrayService.StopXray()
	if err != nil {
		logger.Error("stop xray failed:", err)
		return err
	}

	return nil
}

func (s *ServerService) RestartXrayService() (string error) {
	s.xrayService.StopXray()
	defer func() {
		err := s.xrayService.RestartXray(true)
		if err != nil {
			logger.Error("start xray failed:", err)
		}
	}()

	return nil
}

func (s *ServerService) downloadXRay(version string) (string, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch osName {
	case "darwin":
		osName = "macos"
	}

	switch arch {
	case "amd64":
		arch = "64"
	case "arm64":
		arch = "arm64-v8a"
	case "armv7":
		arch = "arm32-v7a"
	case "armv6":
		arch = "arm32-v6"
	case "armv5":
		arch = "arm32-v5"
	case "386":
		arch = "32"
	case "s390x":
		arch = "s390x"
	}

	fileName := fmt.Sprintf("Xray-%s-%s.zip", osName, arch)
	url := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/%s/%s", version, fileName)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	os.Remove(fileName)
	file, err := os.Create(fileName)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", err
	}

	return fileName, nil
}

func (s *ServerService) UpdateXray(version string) error {
	zipFileName, err := s.downloadXRay(version)
	if err != nil {
		return err
	}

	zipFile, err := os.Open(zipFileName)
	if err != nil {
		return err
	}
	defer func() {
		zipFile.Close()
		os.Remove(zipFileName)
	}()

	stat, err := zipFile.Stat()
	if err != nil {
		return err
	}
	reader, err := zip.NewReader(zipFile, stat.Size())
	if err != nil {
		return err
	}

	s.xrayService.StopXray()
	defer func() {
		err := s.xrayService.RestartXray(true)
		if err != nil {
			logger.Error("start xray failed:", err)
		}
	}()

	copyZipFile := func(zipName string, fileName string) error {
		zipFile, err := reader.Open(zipName)
		if err != nil {
			return err
		}
		os.Remove(fileName)
		file, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_TRUNC, fs.ModePerm)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(file, zipFile)
		return err
	}

	err = copyZipFile("xray", xray.GetBinaryPath())
	if err != nil {
		return err
	}

	return nil
}

func (s *ServerService) GetLogs(count string, level string, syslog string) []string {
	c, _ := strconv.Atoi(count)
	var lines []string

	if syslog == "true" {
		cmdArgs := []string{"journalctl", "-u", "x-ui", "--no-pager", "-n", count, "-p", level}
		// Run the command
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			return []string{"Failed to run journalctl command!"}
		}
		lines = strings.Split(out.String(), "\n")
	} else {
		lines = logger.GetLogs(c, level)
	}

	return lines
}

func (s *ServerService) GetAccessLog(count string, grep string) []string {
	accessLogPath, err := xray.GetAccessLogPath()
	if err != nil {
		return []string{"Error in Access Log retrieval: " + err.Error()}
	}

	if accessLogPath != "none" && accessLogPath != "" {
		var cmdArgs []string
		if grep != "" {
			cmdArgs = []string{"bash", "-c", fmt.Sprintf("tail -n %s %s | grep '%s' | sort -r", count, accessLogPath, grep)}
    	} else {
    		cmdArgs = []string{"bash", "-c", fmt.Sprintf("tail -n %s %s | sort -r", count, accessLogPath)}
    	}
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			return []string{"Failed to run command: " + err.Error()}
		}
		return strings.Split(out.String(), "\n")
	} else {
		return []string{"Access Log disabled!"}
	}
}

func (s *ServerService) GetErrorLog(count string, grep string) []string {
	errorLogPath, err := xray.GetErrorLogPath()
	if err != nil {
		return []string{"Error in Error Log retrieval: " + err.Error()}
	}

	if errorLogPath != "none" && errorLogPath != "" {
		var cmdArgs []string
		if grep != "" {
			cmdArgs = []string{"bash", "-c", fmt.Sprintf("tail -n %s %s | grep '%s' | sort -r", count, errorLogPath, grep)}
    	} else {
    		cmdArgs = []string{"bash", "-c", fmt.Sprintf("tail -n %s %s | sort -r", count, errorLogPath)}
    	}
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			return []string{"Failed to run command: " + err.Error()}
		}
		return strings.Split(out.String(), "\n")
	} else {
		return []string{"Error Log disabled!"}
	}
}

func (s *ServerService) GetConfigJson() (interface{}, error) {
	config, err := s.xrayService.GetXrayConfig()
	if err != nil {
		return nil, err
	}
	contents, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, err
	}

	var jsonData interface{}
	err = json.Unmarshal(contents, &jsonData)
	if err != nil {
		return nil, err
	}

	return jsonData, nil
}

func (s *ServerService) GetDb() ([]byte, error) {
	// Update by manually trigger a checkpoint operation
	err := database.Checkpoint()
	if err != nil {
		return nil, err
	}
	// Open the file for reading
	file, err := os.Open(config.GetDBPath())
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the file contents
	fileContents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return fileContents, nil
}

func (s *ServerService) ImportDB(file multipart.File) error {
	// Check if the file is a SQLite database
	isValidDb, err := database.IsSQLiteDB(file)
	if err != nil {
		return common.NewErrorf("Error checking db file format: %v", err)
	}
	if !isValidDb {
		return common.NewError("Invalid db file format")
	}

	// Reset the file reader to the beginning
	_, err = file.Seek(0, 0)
	if err != nil {
		return common.NewErrorf("Error resetting file reader: %v", err)
	}

	// Save the file as temporary file
	tempPath := fmt.Sprintf("%s.temp", config.GetDBPath())
	// Remove the existing fallback file (if any) before creating one
	_, err = os.Stat(tempPath)
	if err == nil {
		errRemove := os.Remove(tempPath)
		if errRemove != nil {
			return common.NewErrorf("Error removing existing temporary db file: %v", errRemove)
		}
	}
	// Create the temporary file
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return common.NewErrorf("Error creating temporary db file: %v", err)
	}
	defer tempFile.Close()

	// Remove temp file before returning
	defer os.Remove(tempPath)

	// Save uploaded file to temporary file
	_, err = io.Copy(tempFile, file)
	if err != nil {
		return common.NewErrorf("Error saving db: %v", err)
	}

	// Check if we can init db or not
	err = database.InitDB(tempPath)
	if err != nil {
		return common.NewErrorf("Error checking db: %v", err)
	}

	// Stop Xray
	s.StopXrayService()

	// Backup the current database for fallback
	fallbackPath := fmt.Sprintf("%s.backup", config.GetDBPath())
	// Remove the existing fallback file (if any)
	_, err = os.Stat(fallbackPath)
	if err == nil {
		errRemove := os.Remove(fallbackPath)
		if errRemove != nil {
			return common.NewErrorf("Error removing existing fallback db file: %v", errRemove)
		}
	}
	// Move the current database to the fallback location
	err = os.Rename(config.GetDBPath(), fallbackPath)
	if err != nil {
		return common.NewErrorf("Error backing up temporary db file: %v", err)
	}

	// Remove the temporary file before returning
	defer os.Remove(fallbackPath)

	// Move temp to DB path
	err = os.Rename(tempPath, config.GetDBPath())
	if err != nil {
		errRename := os.Rename(fallbackPath, config.GetDBPath())
		if errRename != nil {
			return common.NewErrorf("Error moving db file and restoring fallback: %v", errRename)
		}
		return common.NewErrorf("Error moving db file: %v", err)
	}

	// Migrate DB
	err = database.InitDB(config.GetDBPath())
	if err != nil {
		errRename := os.Rename(fallbackPath, config.GetDBPath())
		if errRename != nil {
			return common.NewErrorf("Error migrating db and restoring fallback: %v", errRename)
		}
		return common.NewErrorf("Error migrating db: %v", err)
	}
	s.inboundService.MigrateDB()

	// Start Xray
	err = s.RestartXrayService()
	if err != nil {
		return common.NewErrorf("Imported DB but Failed to start Xray: %v", err)
	}

	return nil
}

func (s *ServerService) GetNewX25519Cert() (interface{}, error) {
	// Run the command
	cmd := exec.Command(xray.GetBinaryPath(), "x25519")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")

	privateKeyLine := strings.Split(lines[0], ":")
	publicKeyLine := strings.Split(lines[1], ":")

	privateKey := strings.TrimSpace(privateKeyLine[1])
	publicKey := strings.TrimSpace(publicKeyLine[1])

	keyPair := map[string]interface{}{
		"privateKey": privateKey,
		"publicKey":  publicKey,
	}

	return keyPair, nil
}
