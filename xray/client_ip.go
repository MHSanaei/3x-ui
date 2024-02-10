package xray

import (
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"x-ui/logger"
)

var customChain string = "IPLIMIT"
var listener *ClientIPListener

type ClientIPListener struct {
	p                *process
	logWriter        *LogWriter
	InboundClientIps map[string][]string
	blacklist        []string
}

func NewClientIPListener(p *process) *ClientIPListener {
	listener = &ClientIPListener{
		p:                p,
		logWriter:        p.logWriter,
		InboundClientIps: make(map[string][]string),
		blacklist:        make([]string, 0),
	}

	return listener
}

func GetClientIPListener() (*ClientIPListener, error) {
	if listener == nil {
		return nil, errors.New("ClientIPListener is not initialized")
	}

	return listener, nil
}

func (l *ClientIPListener) GetOnlineClients() []string {
	return listener.p.onlineClients
}

func (l *ClientIPListener) processLine(line string) {
	ipRegx, _ := regexp.Compile(`((\d+\.\d+\.\d+\.\d+)|(\[.*\])).* accepted`)
	emailRegx, _ := regexp.Compile(`email:.+`)

	matches := ipRegx.FindStringSubmatch(line)
	if len(matches) > 1 {
		ip := matches[1]
		if ip == "127.0.0.1" || ip == "[::1]" {
			logger.Debug("Skip localhost IP: ", ip)
			return
		}

		matchesEmail := emailRegx.FindString(line)
		if matchesEmail == "" {
			return
		}

		matchesEmail = strings.TrimSpace(strings.Split(matchesEmail, "email: ")[1])
		if l.InboundClientIps[matchesEmail] != nil {
			if l.contains(l.InboundClientIps[matchesEmail], ip) {
				return
			}
			l.InboundClientIps[matchesEmail] = append(l.InboundClientIps[matchesEmail], ip)
		} else {
			l.InboundClientIps[matchesEmail] = append(l.InboundClientIps[matchesEmail], ip)
		}

		logger.Info("Client IP: ", ip, " Email: ", matchesEmail)
	}
}

func (l *ClientIPListener) ProcessBlacklist() {
	unbanCmd := exec.Command("iptables", "-F", customChain)
	_ = unbanCmd.Run()

	for _, ip := range l.blacklist {
		banCmd := exec.Command("iptables", "-A", customChain, "-s", ip, "-j", "DROP")
		_ = banCmd.Run()
	}
}

func (l *ClientIPListener) contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

// Start listening to the xray stdout
func (l *ClientIPListener) Start() {
	l.logWriter.SetListener(l.processLine)
}

func (l *ClientIPListener) UnbanAllIPs() {
	l.blacklist = make([]string, 0)
}

func (l *ClientIPListener) UnbanIP(ip string) {
	for i, v := range l.blacklist {
		if v == ip {
			l.blacklist = append(l.blacklist[:i], l.blacklist[i+1:]...)
			return
		}
	}
}

func (l *ClientIPListener) BanIP(ip string) {
	l.blacklist = append(l.blacklist, ip)
}
