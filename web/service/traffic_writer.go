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
	twCtx    context.Context
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

	if twCancel != nil && twDone != nil {
		select {
		case <-twDone:
			clearTrafficWriterState()
		default:
			return
		}
	}

	queue := make(chan *trafficWriteRequest, trafficWriterQueueSize)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	twQueue = queue
	twCtx = ctx
	twCancel = cancel
	twDone = done

	go runTrafficWriter(ctx, queue, done)
}

// StopTrafficWriter cancels the writer context and waits for the goroutine to
// drain any pending requests before returning. Resets the package state so a
// subsequent StartTrafficWriter can spawn a fresh consumer.
func StopTrafficWriter() {
	twMu.Lock()
	cancel := twCancel
	done := twDone
	if cancel == nil || done == nil {
		twMu.Unlock()
		return
	}
	cancel()
	twMu.Unlock()

	<-done

	twMu.Lock()
	if twDone == done {
		clearTrafficWriterState()
	}
	twMu.Unlock()
}

func clearTrafficWriterState() {
	twQueue = nil
	twCtx = nil
	twCancel = nil
	twDone = nil
}

func runTrafficWriter(ctx context.Context, queue chan *trafficWriteRequest, done chan struct{}) {
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
	req := &trafficWriteRequest{apply: fn, done: make(chan error, 1)}

	twMu.Lock()
	queue := twQueue
	ctx := twCtx
	done := twDone
	if queue == nil || ctx == nil || done == nil {
		twMu.Unlock()
		return safeApply(fn)
	}

	select {
	case <-ctx.Done():
		twMu.Unlock()
		return safeApply(fn)
	default:
	}

	timer := time.NewTimer(trafficWriterSubmitTimeout)
	defer timer.Stop()
	select {
	case queue <- req:
		twMu.Unlock()
	case <-timer.C:
		twMu.Unlock()
		return errors.New("traffic writer queue full")
	}

	select {
	case err := <-req.done:
		return err
	case <-done:
		select {
		case err := <-req.done:
			return err
		default:
			return errors.New("traffic writer stopped before write completed")
		}
	}
}
