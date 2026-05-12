package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/logger"
)

const (
	trafficWriterQueueSize     = 256
	trafficWriterSubmitTimeout = 5 * time.Second
)

type trafficWriteRequest struct {
	apply func() error
	done  chan error
}

var (
	twMu     sync.Mutex
	twQueue  chan *trafficWriteRequest
	twCancel context.CancelFunc
	twDone   chan struct{}
)

// StartTrafficWriter spins up the serial writer goroutine. Safe to call again
// after StopTrafficWriter — each Start/Stop cycle gets fresh channels. The
// previous sync.Once-based implementation deadlocked after a SIGHUP-driven
// panel restart: Stop killed the consumer goroutine but Once prevented Start
// from spawning a new one, so every later submitTrafficWrite blocked forever
// on <-req.done with no consumer (including the AddTraffic call inside
// XrayService.GetXrayConfig that runs from startTask).
func StartTrafficWriter() {
	twMu.Lock()
	defer twMu.Unlock()
	if twQueue != nil {
		return
	}
	queue := make(chan *trafficWriteRequest, trafficWriterQueueSize)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	twQueue = queue
	twCancel = cancel
	twDone = done
	go runTrafficWriter(queue, ctx, done)
}

// StopTrafficWriter cancels the writer context and waits for the goroutine to
// drain any pending requests before returning. Resets the package state so a
// subsequent StartTrafficWriter can spawn a fresh consumer.
func StopTrafficWriter() {
	twMu.Lock()
	cancel := twCancel
	done := twDone
	twQueue = nil
	twCancel = nil
	twDone = nil
	twMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func runTrafficWriter(queue chan *trafficWriteRequest, ctx context.Context, done chan struct{}) {
	defer close(done)
	for {
		select {
		case req := <-queue:
			req.done <- safeApply(req.apply)
		case <-ctx.Done():
			for {
				select {
				case req := <-queue:
					req.done <- safeApply(req.apply)
				default:
					return
				}
			}
		}
	}
}

func safeApply(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("traffic writer panic: %v", r)
			logger.Error(err.Error())
		}
	}()
	return fn()
}

func submitTrafficWrite(fn func() error) error {
	twMu.Lock()
	queue := twQueue
	twMu.Unlock()

	if queue == nil {
		return safeApply(fn)
	}
	req := &trafficWriteRequest{apply: fn, done: make(chan error, 1)}
	select {
	case queue <- req:
	case <-time.After(trafficWriterSubmitTimeout):
		return errors.New("traffic writer queue full")
	}
	return <-req.done
}
