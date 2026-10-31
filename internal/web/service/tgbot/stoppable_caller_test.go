package tgbot

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mymmrac/telego"
	ta "github.com/mymmrac/telego/telegoapi"
	"github.com/valyala/fasthttp"
)

// With Telegram unreachable a send hangs for its whole fasthttp timeout, and
// the event bus, so the panel restart behind it, waited that long.
func TestStopBotEndsInFlightTelegramCall(t *testing.T) {
	arrived := make(chan struct{}, 1)
	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		arrived <- struct{}{}
		<-release
	}))
	t.Cleanup(func() { close(release); hang.Close() })

	stop := make(chan struct{})
	b, err := telego.NewBot(testBotToken, telego.WithAPIServer(hang.URL),
		telego.WithAPICaller(stoppableCaller{inner: ta.FastHTTPCaller{Client: &fasthttp.Client{}}, stopped: stop}))
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}
	origBot := liveBot()
	botMu.Lock()
	bot, botStop = b, stop
	botMu.Unlock()
	t.Cleanup(func() { setTestBot(origBot) })

	errc := make(chan error, 1)
	go func() {
		_, err := b.GetMe(context.Background())
		errc <- err
	}()
	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("getMe never reached the fake Telegram API")
	}

	StopBot()

	select {
	case err := <-errc:
		if !errors.Is(err, errBotStopped) {
			t.Fatalf("getMe returned %v, want errBotStopped", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("getMe still blocked 2s after StopBot")
	}
	if _, err := b.GetMe(context.Background()); !errors.Is(err, errBotStopped) {
		t.Fatalf("call after StopBot returned %v, want errBotStopped", err)
	}
	select {
	case <-arrived:
		t.Fatal("a call after StopBot still went out to Telegram")
	default:
	}
}
