package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// oversizedVlessSettings holds enough clients that the core's 4 MiB API limit
// refuses the inbound as one request, as it does past about 46,000 users.
func oversizedVlessSettings() string {
	var b strings.Builder
	b.WriteString(`{"decryption":"none","clients":[`)
	for i := range 50000 {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"%08d-0000-4000-8000-000000000000","email":"user-%05d@oversized","enable":true,"created_at":1,"updated_at":1}`, i, i)
	}
	b.WriteString(`]}`)
	return b.String()
}

// runOversizedInbound starts a core already serving the inbound, stores its row
// and makes that core the panel's local runtime.
func runOversizedInbound(t *testing.T) (*model.Inbound, func() any) {
	t.Helper()
	setupBulkDB(t)
	ib := &model.Inbound{
		Tag: "oversized", Listen: "127.0.0.1", Port: freeLoopbackPort(t), Protocol: model.VLESS, Enable: true,
		Settings:       oversizedVlessSettings(),
		StreamSettings: `{"network":"tcp","security":"none"}`,
		Sniffing:       `{"enabled":false}`,
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	instance, _, local := localRuntimeOnCore(t, withInbound(isolationConfig(t), ib))
	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
	mgr.SetLocalRuntimeOverride(local)
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })
	return ib, func() any { return coreInboundHandler(t, instance, ib.Tag) }
}

// A save that rebuilds the handler cannot send a 50,000-client inbound through
// the core's API; the handler it serves from must stay up until the restart.
func TestInboundSaveKeepsAHandlerItCannotReplaceLive(t *testing.T) {
	ib, handler := runOversizedInbound(t)
	serving := handler()
	if serving == nil {
		t.Fatal("the core did not start with the inbound")
	}

	form := *ib
	form.Settings = `{"decryption":"none"}`
	form.Sniffing = `{"enabled":true,"destOverride":["http","tls"]}`
	_, needRestart, err := (&InboundService{}).UpdateInbound(&form)
	if err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	if !needRestart {
		t.Fatal("needRestart = false: nothing would apply the new sniffing")
	}
	if handler() != serving {
		t.Fatal("the save removed the running handler and could not add its replacement: the inbound is down until the next restart")
	}
}
