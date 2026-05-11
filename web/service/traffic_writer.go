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
	twQueue  chan *trafficWriteRequest
	twCtx    context.Context
	twCancel context.CancelFunc
	twDone   chan struct{}
	twOnce   sync.Once
)

func StartTrafficWriter() {
	twOnce.Do(func() {
		twQueue = make(chan *trafficWriteRequest, trafficWriterQueueSize)
		twCtx, twCancel = context.WithCancel(context.Background())
		twDone = make(chan struct{})
		go runTrafficWriter()
	})
}

func StopTrafficWriter() {
	if twCancel != nil {
		twCancel()
		<-twDone
	}
}

func runTrafficWriter() {
	defer close(twDone)
	for {
		select {
		case req := <-twQueue:
			req.done <- safeApply(req.apply)
		case <-twCtx.Done():
			for {
				select {
				case req := <-twQueue:
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
	if twQueue == nil {
		return safeApply(fn)
	}
	req := &trafficWriteRequest{apply: fn, done: make(chan error, 1)}
	select {
	case twQueue <- req:
	case <-time.After(trafficWriterSubmitTimeout):
		return errors.New("traffic writer queue full")
	}
	return <-req.done
}
