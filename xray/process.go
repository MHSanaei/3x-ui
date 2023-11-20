package xray

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"x-ui/config"
	"x-ui/logger"
	"x-ui/util/common"

	"github.com/Workiva/go-datastructures/queue"
)

func GetBinaryName() string {
	return fmt.Sprintf("xray-%s-%s", runtime.GOOS, runtime.GOARCH)
}

func GetBinaryPath() string {
	return config.GetBinFolderPath() + "/" + GetBinaryName()
}

func GetConfigPath() string {
	return config.GetBinFolderPath() + "/config.json"
}

func GetGeositePath() string {
	return config.GetBinFolderPath() + "/geosite.dat"
}

func GetGeoipPath() string {
	return config.GetBinFolderPath() + "/geoip.dat"
}

func GetIPLimitLogPath() string {
	return config.GetLogFolder() + "/3xipl.log"
}

func GetIPLimitBannedLogPath() string {
	return config.GetLogFolder() + "/3xipl-banned.log"
}

func GetAccessPersistentLogPath() string {
	return config.GetLogFolder() + "/3xipl-access-persistent.log"
}

func GetAccessLogPath() string {
	config, err := os.ReadFile(GetConfigPath())
	if err != nil {
		logger.Warningf("Something went wrong: %s", err)
	}

	jsonConfig := map[string]interface{}{}
	err = json.Unmarshal([]byte(config), &jsonConfig)
	if err != nil {
		logger.Warningf("Something went wrong: %s", err)
	}

	if jsonConfig["log"] != nil {
		jsonLog := jsonConfig["log"].(map[string]interface{})
		if jsonLog["access"] != nil {

			accessLogPath := jsonLog["access"].(string)

			return accessLogPath
		}
	}
	return ""
}

func stopProcess(p *Process) {
	p.Stop()
}

type Process struct {
	*process
}

func NewProcess(xrayConfig *Config) *Process {
	p := &Process{newProcess(xrayConfig)}
	runtime.SetFinalizer(p, stopProcess)
	return p
}

type process struct {
	cmd *exec.Cmd

	version string
	apiPort int

	config    *Config
	lines     *queue.Queue
	exitErr   error
	startTime time.Time
}

func newProcess(config *Config) *process {
	return &process{
		version:   "Unknown",
		config:    config,
		lines:     queue.New(100),
		startTime: time.Now(),
	}
}

func (p *process) IsRunning() bool {
	return p.cmd != nil && p.cmd.Process != nil && p.cmd.ProcessState == nil
}

func (p *process) GetErr() error {
	return p.exitErr
}

func (p *process) GetResult() string {
	var lines []string
	for !p.lines.Empty() {
		if item, err := p.lines.Get(1); err == nil {
			lines = append(lines, item[0].(string))
		}
	}
	if len(lines) == 0 && p.exitErr != nil {
		return p.exitErr.Error()
	}
	return strings.Join(lines, "\n")
}

func (p *process) GetVersion() string {
	return p.version
}

func (p *Process) GetAPIPort() int {
	return p.apiPort
}

func (p *Process) GetConfig() *Config {
	return p.config
}

func (p *Process) GetUptime() uint64 {
	return uint64(time.Since(p.startTime).Seconds())
}

func (p *process) refreshAPIPort() {
	for _, inbound := range p.config.InboundConfigs {
		if inbound.Tag == "api" {
			p.apiPort = inbound.Port
			break
		}
	}
}

func (p *process) refreshVersion() {
	cmd := exec.Command(GetBinaryPath(), "-version")
	data, err := cmd.Output()
	if err != nil {
		p.version = "Unknown"
	} else {
		datas := bytes.Split(data, []byte(" "))
		if len(datas) <= 1 {
			p.version = "Unknown"
		} else {
			p.version = string(datas[1])
		}
	}
}

func (p *process) Start() error {
	if p.IsRunning() {
		return errors.New("xray is already running")
	}

	data, err := json.MarshalIndent(p.config, "", "  ")
	if err != nil {
		return common.NewErrorf("Failed to generate xray configuration file: %v", err)
	}
	configPath := GetConfigPath()
	if err = os.WriteFile(configPath, data, fs.ModePerm); err != nil {
		return common.NewErrorf("Failed to write configuration file: %v", err)
	}

	p.cmd = exec.Command(GetBinaryPath(), "-c", configPath)

	stdReader, err := p.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	errReader, err := p.cmd.StderrPipe()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)

	startReader := func(reader io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			p.lines.Put(scanner.Text())
		}
	}

	go startReader(stdReader)
	go startReader(errReader)

	go func() {
		defer wg.Wait()
		p.exitErr = p.cmd.Run()
	}()

	p.refreshVersion()
	p.refreshAPIPort()

	return nil
}

func (p *process) Stop() error {
	if !p.IsRunning() {
		return errors.New("xray is not running")
	}
	return p.cmd.Process.Signal(syscall.SIGTERM)
}
