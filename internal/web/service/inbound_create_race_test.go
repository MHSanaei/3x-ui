package service

import (
	"fmt"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// A wildcard listener and a specific one on the same port overlap, but they are
// two different rows: only the in-transaction check can reject the pair, and it
// can only do so if the check and the insert cannot interleave.
func TestAddInboundConcurrentOverlappingListenersSingleWinner(t *testing.T) {
	setupConflictDB(t)

	const rounds = 25
	for round := range rounds {
		port := 24000 + round
		claims := []*model.Inbound{
			{
				Tag: fmt.Sprintf("race-%d-wildcard", round), Listen: "",
				Port: port, Protocol: model.VLESS,
				StreamSettings: `{"network":"tcp"}`, Settings: `{"clients":[]}`,
			},
			{
				Tag: fmt.Sprintf("race-%d-specific", round), Listen: "127.0.0.1",
				Port: port, Protocol: model.Trojan,
				StreamSettings: `{"network":"tcp"}`, Settings: `{"clients":[]}`,
			},
		}

		start := make(chan struct{})
		errs := make(chan error, len(claims))
		var wg sync.WaitGroup
		for _, claim := range claims {
			wg.Add(1)
			go func(inbound *model.Inbound) {
				defer wg.Done()
				<-start
				_, _, err := (&InboundService{}).AddInbound(inbound)
				errs <- err
			}(claim)
		}
		close(start)
		wg.Wait()
		close(errs)

		committed := 0
		rejections := make([]string, 0, len(claims))
		for err := range errs {
			if err == nil {
				committed++
				continue
			}
			rejections = append(rejections, err.Error())
		}
		if committed != 1 {
			t.Fatalf("round %d port %d: concurrent AddInbound committed=%d, want exactly 1 (rejections: %v)",
				round, port, committed, rejections)
		}
	}
}

// Editing an inbound onto a port another one already holds must be rejected —
// the check moved inside the transaction, and nothing else guards this path.
func TestUpdateInboundRejectsPortTakenByAnother(t *testing.T) {
	setupConflictDB(t)

	svc := &InboundService{}
	first := &model.Inbound{
		Tag: "update-holder", Listen: "", Port: 25101, Protocol: model.VLESS,
		StreamSettings: `{"network":"tcp"}`, Settings: `{"clients":[]}`,
	}
	if _, _, err := svc.AddInbound(first); err != nil {
		t.Fatalf("seed holder: %v", err)
	}
	second := &model.Inbound{
		Tag: "update-mover", Listen: "", Port: 25102, Protocol: model.VLESS,
		StreamSettings: `{"network":"tcp"}`, Settings: `{"clients":[]}`,
	}
	if _, _, err := svc.AddInbound(second); err != nil {
		t.Fatalf("seed mover: %v", err)
	}

	second.Port = first.Port
	if _, _, err := svc.UpdateInbound(second); err == nil {
		t.Fatal("moving an inbound onto a port already in use was accepted")
	}

	var stored model.Inbound
	if err := database.GetDB().First(&stored, second.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Port != 25102 {
		t.Fatalf("rejected update still changed the stored port to %d", stored.Port)
	}
}
