package amneziawgnet

import "testing"

// An inbound id past the slot count used to be refused outright, which capped a
// database at 435 AmneziaWG inbounds for its whole life (#6537).
func TestSOCKSPortForInboundKeepsEverySlotInsideTheWindow(t *testing.T) {
	t.Run("no id derives a port outside the window", func(t *testing.T) {
		for _, id := range []int{1, 2, 434, 435, 436, 437, 870, 871, 6537, 70350, 1_000_000} {
			port := SOCKSPortForInbound(id)
			if port < SOCKSBasePort+1 || port > 65535 {
				t.Errorf("id %d derives relay port %d, outside %d..65535", id, port, SOCKSBasePort+1)
			}
		}
	})

	// Every id the old formula reached must keep its exact port, or upgrading
	// moves a running relay. Ids 1..435 also leave SOCKSBasePort itself unused.
	t.Run("ids up to the slot count keep the port they always had", func(t *testing.T) {
		for id := 1; id <= 435; id++ {
			if got, want := SOCKSPortForInbound(id), SOCKSBasePort+id; got != want {
				t.Errorf("id %d moved from relay port %d to %d", id, want, got)
			}
		}
	})
}
