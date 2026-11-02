package tgbot

import (
	"context"
	"errors"

	ta "github.com/mymmrac/telego/telegoapi"
)

var errBotStopped = errors.New("telegram bot stopped")

// stoppableCaller returns as soon as its bot is stopped or the call's context
// ends; fasthttp honours only deadlines, so a blocked route held panel restarts.
type stoppableCaller struct {
	inner   ta.Caller
	stopped <-chan struct{}
}

type callResult struct {
	resp *ta.Response
	err  error
}

func (c stoppableCaller) Call(ctx context.Context, url string, data *ta.RequestData) (*ta.Response, error) {
	select {
	case <-c.stopped:
		return nil, errBotStopped
	default:
	}
	done := make(chan callResult, 1)
	go func() {
		resp, err := c.inner.Call(ctx, url, data)
		done <- callResult{resp, err}
	}()
	select {
	case r := <-done:
		return r.resp, r.err
	case <-c.stopped:
		return nil, errBotStopped
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
