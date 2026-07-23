package service

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type ClientSpeed struct {
	IPs      []string
	DownMbps int
	UpMbps   int
}

type appliedClient struct {
	classID   uint16
	policeIdx uint32
	downMbps  int
	upMbps    int
	ips       map[string]struct{}
	downH     map[string]uint32
	upH       map[string]uint32
}

type TcShaper struct {
	mu         sync.Mutex
	iface      string
	nextClass  uint16
	nextFilter uint32
	nextPolice uint32
	ready      bool
	ownIngress bool
	applied    map[string]*appliedClient
	runner     func(args ...string) error
}

func DetectPrimaryInterface() (string, error) {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Scan()
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == "00000000" {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("tc: no default route found in /proc/net/route")
}

func NewTcShaper(iface string) *TcShaper {
	return &TcShaper{
		iface:      iface,
		nextClass:  1,
		nextFilter: 1,
		nextPolice: 1,
		applied:    make(map[string]*appliedClient),
	}
}

func (s *TcShaper) Init() error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("tc: unsupported on windows")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = s.runTC("qdisc", "del", "dev", s.iface, "root")
	if err := s.runTC("qdisc", "add", "dev", s.iface, "root", "handle", "1:", "htb", "default", "9999"); err != nil {
		return fmt.Errorf("tc init on %s: %w", s.iface, err)
	}
	if err := s.runTC("class", "add", "dev", s.iface, "parent", "1:", "classid", "1:9999", "htb", "rate", "100gbit"); err != nil {
		_ = s.runTC("qdisc", "del", "dev", s.iface, "root")
		return fmt.Errorf("tc default class on %s: %w", s.iface, err)
	}

	s.ownIngress = false
	_ = s.runTC("qdisc", "del", "dev", s.iface, "ingress")
	if err := s.runTC("qdisc", "add", "dev", s.iface, "handle", "ffff:", "ingress"); err != nil {
		logger.Warning("[tc] ingress qdisc unavailable (upload limits disabled):", err)
	} else {
		s.ownIngress = true
	}

	s.nextClass = 1
	s.nextFilter = 1
	s.nextPolice = 1
	s.applied = make(map[string]*appliedClient)
	s.ready = true
	return nil
}

func (s *TcShaper) Sync(clients map[string]ClientSpeed) {
	if runtime.GOOS == "windows" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready {
		return
	}

	desired := make(map[string]ClientSpeed, len(clients))
	for email, speed := range clients {
		if (speed.DownMbps <= 0 && speed.UpMbps <= 0) || len(speed.IPs) == 0 {
			continue
		}
		ips := uniqueValidIPs(speed.IPs)
		if len(ips) == 0 {
			continue
		}
		desired[email] = ClientSpeed{IPs: ips, DownMbps: speed.DownMbps, UpMbps: speed.UpMbps}
	}

	for email, st := range s.applied {
		if _, ok := desired[email]; !ok {
			s.removeClientLocked(email, st)
			delete(s.applied, email)
		}
	}

	for email, speed := range desired {
		st, ok := s.applied[email]
		if !ok {
			s.addClientLocked(email, speed)
			continue
		}
		s.updateClientLocked(email, st, speed)
	}
}

func (s *TcShaper) addClientLocked(email string, speed ClientSpeed) {
	st := &appliedClient{
		ips:   make(map[string]struct{}),
		downH: make(map[string]uint32),
		upH:   make(map[string]uint32),
	}
	if speed.DownMbps > 0 {
		_ = s.ensureDownClassLocked(email, st, speed.DownMbps)
	}
	if speed.UpMbps > 0 && s.ownIngress {
		_ = s.ensureUpPoliceLocked(email, st, speed.UpMbps)
	}
	for _, ip := range speed.IPs {
		st.ips[ip] = struct{}{}
		if st.classID != 0 {
			s.addDownFilterLocked(st, ip)
		}
		if st.policeIdx != 0 {
			s.addUpFilterLocked(st, ip)
		}
	}
	s.applied[email] = st
}

func (s *TcShaper) updateClientLocked(email string, st *appliedClient, speed ClientSpeed) {
	wantIPs := make(map[string]struct{}, len(speed.IPs))
	for _, ip := range speed.IPs {
		wantIPs[ip] = struct{}{}
	}

	if speed.DownMbps > 0 {
		if st.classID == 0 {
			_ = s.ensureDownClassLocked(email, st, speed.DownMbps)
		} else if speed.DownMbps != st.downMbps {
			rate := fmt.Sprintf("%dmbit", speed.DownMbps)
			if err := s.runTC("class", "change", "dev", s.iface, "classid", fmt.Sprintf("1:%x", st.classID), "htb", "rate", rate, "ceil", rate); err != nil {
				logger.Warningf("[tc] class change for %s: %v", email, err)
			} else {
				st.downMbps = speed.DownMbps
			}
		}
	} else if st.classID != 0 {
		for ip, h := range st.downH {
			s.delFilterLocked("1:", h, ip)
			delete(st.downH, ip)
		}
		_ = s.runTC("class", "del", "dev", s.iface, "classid", fmt.Sprintf("1:%x", st.classID))
		st.classID = 0
		st.downMbps = 0
	}

	if speed.UpMbps > 0 && s.ownIngress {
		_ = s.ensureUpPoliceLocked(email, st, speed.UpMbps)
	} else if st.policeIdx != 0 {
		for ip, h := range st.upH {
			s.delFilterLocked("ffff:", h, ip)
			delete(st.upH, ip)
		}
		s.delPoliceLocked(st.policeIdx)
		st.policeIdx = 0
		st.upMbps = 0
	}

	for ip := range st.ips {
		if _, ok := wantIPs[ip]; ok {
			continue
		}
		if h, ok := st.downH[ip]; ok {
			s.delFilterLocked("1:", h, ip)
			delete(st.downH, ip)
		}
		if h, ok := st.upH[ip]; ok {
			s.delFilterLocked("ffff:", h, ip)
			delete(st.upH, ip)
		}
		delete(st.ips, ip)
	}

	for ip := range wantIPs {
		st.ips[ip] = struct{}{}
		if st.classID != 0 {
			if _, ok := st.downH[ip]; !ok {
				s.addDownFilterLocked(st, ip)
			}
		}
		if st.policeIdx != 0 {
			if _, ok := st.upH[ip]; !ok {
				s.addUpFilterLocked(st, ip)
			}
		}
	}
}

func (s *TcShaper) ensureDownClassLocked(email string, st *appliedClient, downMbps int) bool {
	classID, err := s.allocClassLocked()
	if err != nil {
		logger.Warningf("[tc] class for %s: %v", email, err)
		return false
	}
	rate := fmt.Sprintf("%dmbit", downMbps)
	if err := s.runTC("class", "add", "dev", s.iface, "parent", "1:", "classid", fmt.Sprintf("1:%x", classID), "htb", "rate", rate, "ceil", rate); err != nil {
		logger.Warningf("[tc] class for %s: %v", email, err)
		return false
	}
	st.classID = classID
	st.downMbps = downMbps
	return true
}

func (s *TcShaper) ensureUpPoliceLocked(email string, st *appliedClient, upMbps int) bool {
	rate := fmt.Sprintf("%dmbit", upMbps)
	burst := fmt.Sprintf("%dkb", max(upMbps*2, 32))
	if st.policeIdx != 0 {
		if st.upMbps == upMbps {
			return true
		}
		idx := strconv.FormatUint(uint64(st.policeIdx), 10)
		if err := s.runTC("actions", "change", "action", "police", "rate", rate, "burst", burst, "conform-exceed", "drop", "index", idx); err != nil {
			logger.Warningf("[tc] police change for %s: %v", email, err)
			return false
		}
		st.upMbps = upMbps
		return true
	}
	idxNum, err := s.allocPoliceLocked()
	if err != nil {
		logger.Warningf("[tc] police for %s: %v", email, err)
		return false
	}
	idx := strconv.FormatUint(uint64(idxNum), 10)
	if err := s.runTC("actions", "add", "action", "police", "rate", rate, "burst", burst, "conform-exceed", "drop", "index", idx); err != nil {
		logger.Warningf("[tc] police for %s: %v", email, err)
		return false
	}
	st.policeIdx = idxNum
	st.upMbps = upMbps
	return true
}

func (s *TcShaper) removeClientLocked(email string, st *appliedClient) {
	for ip, h := range st.downH {
		s.delFilterLocked("1:", h, ip)
	}
	for ip, h := range st.upH {
		s.delFilterLocked("ffff:", h, ip)
	}
	if st.classID != 0 {
		if err := s.runTC("class", "del", "dev", s.iface, "classid", fmt.Sprintf("1:%x", st.classID)); err != nil {
			logger.Warningf("[tc] class del for %s: %v", email, err)
		}
	}
	if st.policeIdx != 0 {
		s.delPoliceLocked(st.policeIdx)
	}
}

func (s *TcShaper) delPoliceLocked(idx uint32) {
	_ = s.runTC("actions", "delete", "action", "police", "index", strconv.FormatUint(uint64(idx), 10))
}

func (s *TcShaper) allocClassLocked() (uint16, error) {
	for tried := 0; tried < 0xfffe; tried++ {
		id := s.nextClass
		s.nextClass++
		if s.nextClass == 0 || s.nextClass == 9999 {
			s.nextClass = 1
		}
		if id == 0 || id == 9999 {
			continue
		}
		used := false
		for _, st := range s.applied {
			if st.classID == id {
				used = true
				break
			}
		}
		if !used {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no free HTB class id")
}

func (s *TcShaper) allocPoliceLocked() (uint32, error) {
	for tried := uint32(0); tried < ^uint32(0); tried++ {
		id := s.nextPolice
		s.nextPolice++
		if s.nextPolice == 0 {
			s.nextPolice = 1
		}
		if id == 0 {
			continue
		}
		used := false
		for _, st := range s.applied {
			if st.policeIdx == id {
				used = true
				break
			}
		}
		if !used {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no free police action index")
}

func (s *TcShaper) addDownFilterLocked(st *appliedClient, ip string) {
	proto, matchFamily, matchDir, prefix := ipMatch(ip, "dst")
	if proto == "" {
		return
	}
	h := s.nextFilter
	s.nextFilter++
	handle := fmt.Sprintf("800::%x", h)
	args := []string{
		"filter", "add", "dev", s.iface, "protocol", proto, "parent", "1:",
		"prio", "1", "handle", handle, "u32",
		"match", matchFamily, matchDir, ip + prefix, "flowid", fmt.Sprintf("1:%x", st.classID),
	}
	if err := s.runTC(args...); err != nil {
		logger.Warningf("[tc] down filter %s: %v", ip, err)
		return
	}
	st.downH[ip] = h
}

func (s *TcShaper) addUpFilterLocked(st *appliedClient, ip string) {
	proto, matchFamily, matchDir, prefix := ipMatch(ip, "src")
	if proto == "" {
		return
	}
	h := s.nextFilter
	s.nextFilter++
	handle := fmt.Sprintf("800::%x", h)
	idx := strconv.FormatUint(uint64(st.policeIdx), 10)
	args := []string{
		"filter", "add", "dev", s.iface, "parent", "ffff:", "protocol", proto,
		"prio", "1", "handle", handle, "u32",
		"match", matchFamily, matchDir, ip + prefix,
		"action", "police", "index", idx,
	}
	if err := s.runTC(args...); err != nil {
		logger.Warningf("[tc] up filter %s: %v", ip, err)
		return
	}
	st.upH[ip] = h
}

func (s *TcShaper) delFilterLocked(parent string, handle uint32, ip string) {
	proto, _, _, _ := ipMatch(ip, "dst")
	if proto == "" {
		proto = "ip"
	}
	h := fmt.Sprintf("800::%x", handle)
	_ = s.runTC("filter", "del", "dev", s.iface, "protocol", proto, "parent", parent, "prio", "1", "handle", h, "u32")
}

func ipMatch(ip, dir string) (proto, family, matchDir, prefix string) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", "", "", ""
	}
	if parsed.To4() != nil {
		return "ip", "ip", dir, "/32"
	}
	return "ipv6", "ip6", dir, "/128"
}

func uniqueValidIPs(ips []string) []string {
	seen := make(map[string]struct{}, len(ips))
	out := make([]string, 0, len(ips))
	for _, raw := range ips {
		ip := strings.TrimSpace(raw)
		if net.ParseIP(ip) == nil {
			continue
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		out = append(out, ip)
	}
	return out
}

func (s *TcShaper) Cleanup() {
	if runtime.GOOS == "windows" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready {
		return
	}
	for _, st := range s.applied {
		if st.policeIdx != 0 {
			s.delPoliceLocked(st.policeIdx)
		}
	}
	if s.iface != "" {
		_ = s.runTC("qdisc", "del", "dev", s.iface, "root")
		if s.ownIngress {
			_ = s.runTC("qdisc", "del", "dev", s.iface, "ingress")
		}
	}
	s.applied = make(map[string]*appliedClient)
	s.ready = false
	s.ownIngress = false
}

func (s *TcShaper) runTC(args ...string) error {
	if s.runner != nil {
		return s.runner(args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tc", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("tc timed out after 5s: %w", ctx.Err())
		}
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
