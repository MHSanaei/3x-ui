package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"gorm.io/gorm"
)

const (
	trafficWriterQueueSize     = 256
	trafficWriterSubmitTimeout = 5 * time.Second
)

type trafficWriteRequest struct {
	apply func() error
	done  chan error
}

type serializedTxContextKey struct{}

var (
	twMu         sync.Mutex
	twAdminQueue chan *trafficWriteRequest
	twBgQueue    chan *trafficWriteRequest
	twCtx        context.Context
	twCancel     context.CancelFunc
	twDone       chan struct{}
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

	adminQueue := make(chan *trafficWriteRequest, trafficWriterQueueSize)
	bgQueue := make(chan *trafficWriteRequest, trafficWriterQueueSize)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	twAdminQueue = adminQueue
	twBgQueue = bgQueue
	twCtx = ctx
	twCancel = cancel
	twDone = done

	go runTrafficWriter(ctx, adminQueue, bgQueue, done)
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
	twAdminQueue = nil
	twBgQueue = nil
	twCtx = nil
	twCancel = nil
	twDone = nil
}

// trafficWriterQueueLen is the test's view of both lanes' pending requests.
func trafficWriterQueueLen() int {
	twMu.Lock()
	defer twMu.Unlock()
	return len(twAdminQueue) + len(twBgQueue)
}

// One goroutine serves both lanes (no two writes side by side — the Postgres
// deadlock guard c5d31de4e; the admin lane is preferred over the 5s poll and node merges, #6035).
func runTrafficWriter(ctx context.Context, adminQueue, bgQueue chan *trafficWriteRequest, done chan struct{}) {
	defer close(done)
	for {
		select {
		case req := <-adminQueue:
			req.done <- safeApply(req.apply)
		case <-ctx.Done():
			drainTrafficWriter(adminQueue, bgQueue)
			return
		default:
			select {
			case req := <-adminQueue:
				req.done <- safeApply(req.apply)
			case req := <-bgQueue:
				req.done <- safeApply(req.apply)
			case <-ctx.Done():
				drainTrafficWriter(adminQueue, bgQueue)
				return
			}
		}
	}
}

// drainTrafficWriter finishes every request already queued on shutdown, as the
// single-queue writer used to: both lanes drain until empty, so no write is dropped.
func drainTrafficWriter(adminQueue, bgQueue chan *trafficWriteRequest) {
	for {
		drained := false
		select {
		case req := <-adminQueue:
			req.done <- safeApply(req.apply)
			drained = true
		default:
		}
		select {
		case req := <-bgQueue:
			req.done <- safeApply(req.apply)
			drained = true
		default:
		}
		if !drained {
			return
		}
	}
}

// runSerializedTx runs fn inside one DB transaction on the shared serial
// traffic-writer goroutine, so it can never execute concurrently with the
// @every 5s traffic poll (AddTraffic). Both touch the hot client_traffics and
// inbounds rows, and they acquire them in opposite order (the poll locks
// inbounds then client_traffics; an admin client/inbound mutation does the
// reverse), which Postgres aborts as a deadlock (SQLSTATE 40P01). Routing every
// such mutation through this single writer removes that contention entirely.
//
// Keep network I/O (node pushes) OUT of fn: holding the single writer across a
// remote node call would stall all traffic accounting for up to the remote
// timeout. Apply runtime changes after this returns.
func runSerializedTx(fn func(tx *gorm.DB) error) error {
	return submitTrafficWrite(func() error {
		state := &serializedTx{}
		err := database.GetDB().Transaction(func(tx *gorm.DB) error {
			ctx := context.WithValue(tx.Statement.Context, serializedTxContextKey{}, state)
			return fn(tx.WithContext(ctx))
		})
		if err == nil {
			// Still on the writer, so no node merge runs between the commit and the lift.
			withdrawClientTombstones(state.createdClientEmails...)
		}
		return err
	})
}

// serializedTx carries what may only take effect once its transaction commits.
type serializedTx struct {
	createdClientEmails []string
}

func serializedTxOf(tx *gorm.DB) *serializedTx {
	if tx == nil || tx.Statement == nil || tx.Statement.Context == nil {
		return nil
	}
	state, _ := tx.Statement.Context.Value(serializedTxContextKey{}).(*serializedTx)
	return state
}

func isSerializedTx(tx *gorm.DB) bool {
	return serializedTxOf(tx) != nil
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

// submitTrafficWrite runs fn on the serial writer's admin lane, ahead of any
// queued periodic work (the traffic poll, the node sync merges).
func submitTrafficWrite(fn func() error) error {
	return submitTrafficWriteLane(fn, false)
}

// submitTrafficWriteBackground runs fn on the periodic lane (the 5s traffic poll
// and node snapshot merges), served only when no admin-lane write is queued.
func submitTrafficWriteBackground(fn func() error) error {
	return submitTrafficWriteLane(fn, true)
}

func submitTrafficWriteLane(fn func() error, background bool) error {
	req := &trafficWriteRequest{apply: fn, done: make(chan error, 1)}

	twMu.Lock()
	queue := twAdminQueue
	ctx := twCtx
	done := twDone
	if background {
		queue = twBgQueue
	}
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
