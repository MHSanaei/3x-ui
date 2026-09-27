package tgbot

import (
	"sync"
	"testing"
)

// Regression test: writers replace adminIds and isRunning under tgBotMutex, so
// a bare read of either is reported by -race (CI's `race` job).
func TestAdminListReadersShareTheWriterLock(t *testing.T) {
	mock, _ := staleButtonServer(t, map[string]any{
		"sendMessage": map[string]any{"ok": true, "result": map[string]any{
			"message_id": 1,
			"date":       0,
			"chat":       map[string]any{"id": 1, "type": "private"},
		}},
	})
	swapTestBot(t, mock.URL)
	defer mock.Close()

	tgBotMutex.Lock()
	origAdmins := adminIds
	origRunning := isRunning
	tgBotMutex.Unlock()
	t.Cleanup(func() {
		tgBotMutex.Lock()
		adminIds = origAdmins
		isRunning = origRunning
		tgBotMutex.Unlock()
	})

	stop := make(chan struct{})
	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			// The same lock order Start and Stop write with.
			tgBotMutex.Lock()
			if i%2 == 0 {
				adminIds = []int64{111, 222, 333}
			} else {
				adminIds = nil
			}
			isRunning = i%2 == 0
			tgBotMutex.Unlock()
		}
	}()

	tb := &Tgbot{}
	var readers sync.WaitGroup
	for range 4 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for range 300 {
				checkAdmin(111)
				_ = tb.IsRunning()
				// SendMsgToTgbot reads isRunning itself; the mock bot keeps
				// the live path cheap enough to run under -race.
				tb.SendMsgToTgbot(1, "tick")
			}
		}()
	}
	readers.Wait()
	close(stop)
	writer.Wait()
}
