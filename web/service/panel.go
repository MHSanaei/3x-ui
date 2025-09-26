package service

import (
	"os"
	"syscall"
	"time"

	"x-ui/logger"
)

type PanelService struct{}

func (s *PanelService) RestartPanel(delay time.Duration) error {
	p, err := os.FindProcess(syscall.Getpid())
	if err != nil {
		return err
	}
	go func() {
		time.Sleep(delay)
		err := p.Signal(syscall.SIGHUP)
		if err != nil {
			logger.Error("failed to send SIGHUP signal:", err)
		}
	}()
	return nil
}
