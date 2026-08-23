package service

import (
	"fmt"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Concurrent creates on two inbounds hold two different lockInbound mutexes,
// so only the serialized writer's in-tx re-check can reject the second claim.
func TestAddInboundClientConcurrentCrossInboundAddressSingleWinner(t *testing.T) {
	setupBulkDB(t)
	svc := &ClientService{}
	inboundSvc := &InboundService{}

	ib1 := mkInbound(t, 52210, model.WireGuard, wgServerSettings())
	ib2 := mkInbound(t, 52211, model.WireGuard, wgServerSettings())

	const rounds = 25
	for round := range rounds {
		addr := fmt.Sprintf("10.77.%d.7/32", round)
		claims := []*model.Inbound{
			{Id: ib1.Id, Protocol: model.WireGuard, Settings: clientsSettings(t, []model.Client{
				{Email: fmt.Sprintf("race-%d-a@wg", round), Enable: true, AllowedIPs: []string{addr}},
			})},
			{Id: ib2.Id, Protocol: model.WireGuard, Settings: clientsSettings(t, []model.Client{
				{Email: fmt.Sprintf("race-%d-b@wg", round), Enable: true, AllowedIPs: []string{addr}},
			})},
		}

		start := make(chan struct{})
		errs := make(chan error, len(claims))
		var wg sync.WaitGroup
		for _, claim := range claims {
			wg.Add(1)
			go func(data *model.Inbound) {
				defer wg.Done()
				<-start
				_, err := svc.AddInboundClient(inboundSvc, data)
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
			t.Fatalf("round %d addr %s: concurrent AddInboundClient committed=%d, want exactly 1 (rejections: %v)",
				round, addr, committed, rejections)
		}
	}
}
