package controller

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	tgBotServiceName = "xray-bot"
	tgBotEnvPath     = "/usr/local/x-ui/xray-bot/src/.env"

	// Источник установки жёстко зашит — не настраивается и не принимается из UI.
	tgBotRepoURL    = "https://github.com/KimaruBs/3x-ui.git"
	tgBotRawScript  = "https://raw.githubusercontent.com/KimaruBs/3x-ui/main/xray-bot.sh"
	tgBotDir        = "/usr/local/x-ui/xray-bot"
	tgBotScriptPath = "/usr/bin/xray-bot"
	tgBotUnitPath   = "/etc/systemd/system/xray-bot.service"
)

type TgBotController struct {
	BaseController
}

func NewTgBotController(g *gin.RouterGroup) *TgBotController {
	a := &TgBotController{}
	a.initRouter(g)
	return a
}

func (a *TgBotController) initRouter(g *gin.RouterGroup) {
	gg := g.Group("/tgbot")

	gg.GET("/status", a.getStatus)
	gg.POST("/start", a.start)
	gg.POST("/stop", a.stop)
	gg.POST("/restart", a.restart)

	gg.GET("/env", a.getEnv)
	gg.POST("/env", a.setEnv)
	gg.GET("/env/raw", a.getEnvRaw)
	gg.POST("/env/raw", a.setEnvRaw)

	gg.GET("/dependencies", a.checkDependencies)
	gg.GET("/installed", a.checkInstalled)
	gg.POST("/install", a.installBot)
}

// ---------------------------------------------------------------------------
// Статус службы
// ---------------------------------------------------------------------------

func isServiceActive(name string) bool {
	cmd := exec.Command("systemctl", "is-active", "--quiet", name)
	return cmd.Run() == nil
}

func (a *TgBotController) getStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj": gin.H{
			"running": isServiceActive(tgBotServiceName),
		},
	})
}

// ---------------------------------------------------------------------------
// Управление службой: start / stop / restart
// ---------------------------------------------------------------------------

func runSystemctl(action, service string) error {
	cmd := exec.Command("systemctl", action, service)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err.Error(), strings.TrimSpace(string(out)))
	}
	return nil
}

func (a *TgBotController) start(c *gin.Context) {
	if err := runSystemctl("start", tgBotServiceName); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": gin.H{"running": true}})
}

func (a *TgBotController) stop(c *gin.Context) {
	if err := runSystemctl("stop", tgBotServiceName); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": gin.H{"running": false}})
}

func (a *TgBotController) restart(c *gin.Context) {
	if err := runSystemctl("restart", tgBotServiceName); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": gin.H{"running": isServiceActive(tgBotServiceName)}})
}

// ---------------------------------------------------------------------------
// .env: структурный режим — редактирование только уже существующих ключей
// ---------------------------------------------------------------------------

func readEnvMap() (map[string]string, []string, error) {
	f, err := os.Open(tgBotEnvPath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	result := map[string]string{}
	var order []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.Index(trimmed, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		val := strings.TrimSpace(trimmed[idx+1:])
		result[key] = val
		order = append(order, key)
	}
	return result, order, scanner.Err()
}

func (a *TgBotController) getEnv(c *gin.Context) {
	values, order, err := readEnvMap()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": gin.H{"values": values, "order": order}})
}

type setEnvRequest struct {
	Values map[string]string `json:"values"`
}

func (a *TgBotController) setEnv(c *gin.Context) {
	var req setEnvRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": "invalid payload"})
		return
	}

	raw, err := os.ReadFile(tgBotEnvPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": err.Error()})
		return
	}

	lines := strings.Split(string(raw), "\n")
	rejected := []string{}

	for key, newVal := range req.Values {
		found := false
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") || trimmed == "" {
				continue
			}
			idx := strings.Index(trimmed, "=")
			if idx < 0 {
				continue
			}
			existingKey := strings.TrimSpace(trimmed[:idx])
			if existingKey == key {
				lines[i] = fmt.Sprintf("%s=%s", key, newVal)
				found = true
				break
			}
		}
		// Строгий режим: новых ключей не создаём, только правим существующие.
		if !found {
			rejected = append(rejected, key)
		}
	}

	if len(rejected) > 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"msg":     fmt.Sprintf("параметры отсутствуют в .env и не были созданы: %s", strings.Join(rejected, ", ")),
		})
		return
	}

	if err := os.WriteFile(tgBotEnvPath, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---------------------------------------------------------------------------
// .env: сырой режим — полная свобода редактирования файла
// ---------------------------------------------------------------------------

func (a *TgBotController) getEnvRaw(c *gin.Context) {
	raw, err := os.ReadFile(tgBotEnvPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": gin.H{"content": string(raw)}})
}

type setEnvRawRequest struct {
	Content string `json:"content"`
}

func (a *TgBotController) setEnvRaw(c *gin.Context) {
	var req setEnvRawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": "invalid payload"})
		return
	}
	if err := os.WriteFile(tgBotEnvPath, []byte(req.Content), 0600); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---------------------------------------------------------------------------
// Проверка зависимостей
// ---------------------------------------------------------------------------

type depCheck struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
}

func checkCommand(name string, versionArgs ...string) depCheck {
	path, err := exec.LookPath(name)
	if err != nil {
		return depCheck{Name: name, Available: false}
	}
	detail := path
	if len(versionArgs) > 0 {
		out, err := exec.Command(name, versionArgs...).CombinedOutput()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			if len(lines) > 0 {
				detail = strings.TrimSpace(lines[0])
			}
		}
	}
	return depCheck{Name: name, Available: true, Detail: detail}
}

func checkPythonVenv() depCheck {
	if _, err := exec.LookPath("python3"); err != nil {
		return depCheck{Name: "python3-venv", Available: false}
	}
	out, err := exec.Command("python3", "-c", "import venv").CombinedOutput()
	if err != nil {
		return depCheck{Name: "python3-venv", Available: false, Detail: strings.TrimSpace(string(out))}
	}
	return depCheck{Name: "python3-venv", Available: true}
}

func (a *TgBotController) checkDependencies(c *gin.Context) {
	deps := []depCheck{
		checkCommand("git", "--version"),
		checkCommand("python3", "--version"),
		checkPythonVenv(),
		checkCommand("systemctl", "--version"),
	}
	allOk := true
	for _, d := range deps {
		if !d.Available {
			allOk = false
			break
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj": gin.H{
			"dependencies": deps,
			"allSatisfied": allOk,
		},
	})
}

// ---------------------------------------------------------------------------
// Проверка, установлен ли бот
// ---------------------------------------------------------------------------

func (a *TgBotController) checkInstalled(c *gin.Context) {
	appPath := fmt.Sprintf("%s/src/app.py", tgBotDir)
	venvPath := fmt.Sprintf("%s/venv/bin/python3", tgBotDir)
	envPath := fmt.Sprintf("%s/src/.env", tgBotDir)

	_, appErr := os.Stat(appPath)
	_, venvErr := os.Stat(venvPath)
	_, envErr := os.Stat(envPath)
	_, unitErr := os.Stat(tgBotUnitPath)

	installed := appErr == nil && venvErr == nil && unitErr == nil

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"obj": gin.H{
			"installed": installed,
			"hasApp":    appErr == nil,
			"hasVenv":   venvErr == nil,
			"hasEnv":    envErr == nil,
			"hasUnit":   unitErr == nil,
		},
	})
}

// ---------------------------------------------------------------------------
// Установка бота — только из фиксированного репозитория KimaruBs/3x-ui
// ---------------------------------------------------------------------------

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runShell(script string) (string, error) {
	cmd := exec.Command("bash", "-c", script)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (a *TgBotController) installBot(c *gin.Context) {
	var logBuf strings.Builder

	step := func(label string, fn func() (string, error)) bool {
		logBuf.WriteString(fmt.Sprintf("→ %s\n", label))
		out, err := fn()
		if out != "" {
			logBuf.WriteString(out + "\n")
		}
		if err != nil {
			logBuf.WriteString(fmt.Sprintf("ОШИБКА: %s\n", err.Error()))
			return false
		}
		return true
	}

	tmpDir := tgBotDir + "_tmp"
	os.RemoveAll(tmpDir) // на случай обломанной предыдущей попытки

	ok := step("Клонирование репозитория "+tgBotRepoURL, func() (string, error) {
		return runCmd("git", "clone", "--depth", "1", tgBotRepoURL, tmpDir)
	})
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": logBuf.String()})
		return
	}

	ok = step("Копирование файлов бота", func() (string, error) {
		return runShell(fmt.Sprintf(`mkdir -p %q && cp -r %q/xray-bot/* %q/`, tgBotDir, tmpDir, tgBotDir))
	})
	if ok {
		step("Загрузка скрипта управления xray-bot", func() (string, error) {
			return runCmd("wget", "-N", "--no-check-certificate", "-O", tgBotScriptPath, tgBotRawScript)
		})
		runCmd("chmod", "+x", tgBotScriptPath)
	}
	os.RemoveAll(tmpDir)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": logBuf.String()})
		return
	}

	reqPath := tgBotDir + "/requirements.txt"
	if _, err := os.Stat(reqPath); err == nil {
		ok = step("Создание venv", func() (string, error) {
			return runCmd("python3", "-m", "venv", tgBotDir+"/venv")
		})
		if ok {
			step("Обновление pip", func() (string, error) {
				return runCmd(tgBotDir+"/venv/bin/pip", "install", "--upgrade", "pip", "-q")
			})
			ok = step("Установка зависимостей requirements.txt", func() (string, error) {
				return runCmd(tgBotDir+"/venv/bin/pip", "install", "-r", reqPath, "-q")
			})
		}
		if !ok {
			c.JSON(http.StatusOK, gin.H{"success": false, "msg": logBuf.String()})
			return
		}
	}

	unit := fmt.Sprintf(`[Unit]
Description=3x-ui Xray Telegram Bot
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=%s/src
ExecStart=%s/venv/bin/python3 app.py
Restart=on-failure
RestartSec=3s

[Install]
WantedBy=multi-user.target
`, tgBotDir, tgBotDir)

	ok = step("Запись systemd unit-файла", func() (string, error) {
		return "", os.WriteFile(tgBotUnitPath, []byte(unit), 0644)
	})
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "msg": logBuf.String()})
		return
	}

	step("daemon-reload + enable + restart", func() (string, error) {
		out1, _ := runCmd("systemctl", "daemon-reload")
		out2, _ := runCmd("systemctl", "enable", tgBotServiceName)
		out3, err3 := runCmd("systemctl", "restart", tgBotServiceName)
		return out1 + out2 + out3, err3
	})

	logBuf.WriteString("✔ Установка завершена.\n")
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": gin.H{"log": logBuf.String()}})
}
