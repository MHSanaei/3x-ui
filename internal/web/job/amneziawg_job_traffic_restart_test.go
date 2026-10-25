package job

import (
	"errors"
	"strconv"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/amneziawg"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgkern"
	"github.com/mhsanaei/3x-ui/v3/internal/amneziawgnet"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type kernelTrafficTestManager struct {
	id    int
	state amneziawgkern.TrafficInterfaceState
}

func (m *kernelTrafficTestManager) ManagedIDs() []int { return []int{m.id} }

func (m *kernelTrafficTestManager) TrafficStateFor(int) (amneziawgkern.TrafficInterfaceState, bool) {
	return m.state, true
}

func (m *kernelTrafficTestManager) PrimeAdoptedTrafficBaseline(_ int, generation uint64, stats []amneziawgkern.PeerStat, collector *amneziawgkern.Collector) bool {
	if !m.state.Adopted || m.state.Generation != generation {
		return false
	}
	collector.Prime(m.state.Name, stats)
	m.state.Adopted = false
	return true
}

func TestKernelTrafficJobDoesNotRechargeAfterAdoptingInterface(t *testing.T) {
	setupIntegrationDB(t)
	db := database.GetDB()
	inbound := &model.Inbound{
		UserId: 1, Tag: "kernel-restart", Enable: true, Port: 45201,
		Protocol: model.AmneziaWG, Settings: `{"clients":[]}`,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	const email = "kernel-restart@example.test"
	if err := db.Create(&xray.ClientTraffic{InboundId: inbound.Id, Email: email, Enable: true}).Error; err != nil {
		t.Fatalf("create client traffic: %v", err)
	}

	const (
		iface     = "awg-restart"
		publicKey = "peer-public-key"
	)
	desired := []amneziawgkern.Desired{{Instance: amneziawg.Instance{
		Id: inbound.Id, Peers: []amneziawg.Peer{{Email: email, PublicKey: publicKey}},
	}}}
	firstCollector := amneziawgkern.NewCollector()
	firstManager := &kernelTrafficTestManager{id: inbound.Id, state: amneziawgkern.TrafficInterfaceState{Name: iface, Generation: 1}}
	current := []amneziawgkern.PeerStat{{PublicKey: publicKey, RxBytes: 100, TxBytes: 200}}
	collect := func(manager *kernelTrafficTestManager, collector *amneziawgkern.Collector, dump kernelTrafficDumper) {
		t.Helper()
		collectKernelTrafficWith(&service.InboundService{}, desired, manager, collector, dump)
	}
	collect(firstManager, firstCollector, func(string) ([]amneziawgkern.PeerStat, error) { return current, nil })

	// The panel restarts while the kernel interface keeps its cumulative counters.
	restartedCollector := amneziawgkern.NewCollector()
	restartedManager := &kernelTrafficTestManager{id: inbound.Id, state: amneziawgkern.TrafficInterfaceState{
		Name: iface, Generation: 2, Adopted: true,
	}}
	collect(restartedManager, restartedCollector, func(string) ([]amneziawgkern.PeerStat, error) {
		return nil, errors.New("temporary dump error")
	})
	collect(restartedManager, restartedCollector, func(string) ([]amneziawgkern.PeerStat, error) { return nil, nil })
	if !restartedManager.state.Adopted {
		t.Fatal("failed and empty dumps consumed the adopted baseline")
	}
	current = []amneziawgkern.PeerStat{{PublicKey: publicKey, RxBytes: 125, TxBytes: 250}}
	collect(restartedManager, restartedCollector, func(string) ([]amneziawgkern.PeerStat, error) { return current, nil })
	var afterBaseline xray.ClientTraffic
	if err := db.Where("email = ?", email).First(&afterBaseline).Error; err != nil {
		t.Fatalf("reload traffic after baseline: %v", err)
	}
	if afterBaseline.Up != 100 || afterBaseline.Down != 200 {
		t.Fatalf("first adopted dump charged traffic: up %d down %d, want unchanged 100/200", afterBaseline.Up, afterBaseline.Down)
	}
	if restartedManager.state.Adopted {
		t.Fatal("successful nonempty dump did not consume the adopted baseline")
	}
	current = []amneziawgkern.PeerStat{{PublicKey: publicKey, RxBytes: 150, TxBytes: 300}}
	collect(restartedManager, restartedCollector, func(string) ([]amneziawgkern.PeerStat, error) { return current, nil })

	var got xray.ClientTraffic
	if err := db.Where("email = ?", email).First(&got).Error; err != nil {
		t.Fatalf("reload client traffic: %v", err)
	}
	if got.Up != 125 || got.Down != 250 {
		t.Fatalf("usage after adopted-interface restart = up %d down %d, want 125/250 (100/200 before restart plus 25/50 after the baseline)", got.Up, got.Down)
	}

	const newEmail = "kernel-new-peer@example.test"
	if err := db.Create(&xray.ClientTraffic{InboundId: inbound.Id, Email: newEmail, Enable: true}).Error; err != nil {
		t.Fatalf("create new peer traffic: %v", err)
	}
	const newPublicKey = "new-peer-public-key"
	desired[0].Instance.Peers = append(desired[0].Instance.Peers, amneziawg.Peer{Email: newEmail, PublicKey: newPublicKey})
	current = []amneziawgkern.PeerStat{
		{PublicKey: publicKey, RxBytes: 150, TxBytes: 300},
		{PublicKey: newPublicKey, RxBytes: 7, TxBytes: 9},
	}
	collect(restartedManager, restartedCollector, func(string) ([]amneziawgkern.PeerStat, error) { return current, nil })
	var newPeerTraffic xray.ClientTraffic
	if err := db.Where("email = ?", newEmail).First(&newPeerTraffic).Error; err != nil {
		t.Fatalf("reload new peer traffic: %v", err)
	}
	if newPeerTraffic.Up != 7 || newPeerTraffic.Down != 9 {
		t.Fatalf("new peer first counters = up %d down %d, want 7/9", newPeerTraffic.Up, newPeerTraffic.Down)
	}
}

// TestKernelTrafficChargesPeersLeavingTheDesiredSet: reconcile removes a
// disabled peer or inbound from the kernel, so its last counters are read first.
func TestKernelTrafficChargesPeersLeavingTheDesiredSet(t *testing.T) {
	cases := map[string]func(id int) []amneziawgkern.Desired{
		"peer disabled": func(id int) []amneziawgkern.Desired {
			return []amneziawgkern.Desired{{Instance: amneziawg.Instance{Id: id}}}
		},
		"inbound removed": func(int) []amneziawgkern.Desired { return nil },
	}
	port := 45210
	for name, nextDesired := range cases {
		t.Run(name, func(t *testing.T) {
			setupIntegrationDB(t)
			db := database.GetDB()
			port++
			inbound := &model.Inbound{
				UserId: 1, Tag: "kernel-leaving-" + strconv.Itoa(port), Enable: true, Port: port,
				Protocol: model.AmneziaWG, Settings: `{"clients":[]}`,
			}
			if err := db.Create(inbound).Error; err != nil {
				t.Fatalf("create inbound: %v", err)
			}
			const email = "kernel-leaving@example.test"
			if err := db.Create(&xray.ClientTraffic{InboundId: inbound.Id, Email: email, Enable: true}).Error; err != nil {
				t.Fatalf("create client traffic: %v", err)
			}
			const publicKey = "leaving-peer-key"
			manager := &kernelTrafficTestManager{id: inbound.Id, state: amneziawgkern.TrafficInterfaceState{Name: "xui-awg-leaving", Generation: 1}}
			collector := amneziawgkern.NewCollector()
			dump := func(rx, tx int64) kernelTrafficDumper {
				return func(string) ([]amneziawgkern.PeerStat, error) {
					return []amneziawgkern.PeerStat{{PublicKey: publicKey, RxBytes: rx, TxBytes: tx}}, nil
				}
			}
			first := []amneziawgkern.Desired{{Instance: amneziawg.Instance{
				Id: inbound.Id, Peers: []amneziawg.Peer{{Email: email, PublicKey: publicKey}},
			}}}
			collectKernelTrafficWith(&service.InboundService{}, first, manager, collector, dump(100, 200))
			collectKernelTrafficWith(&service.InboundService{}, nextDesired(inbound.Id), manager, collector, dump(150, 260))

			var got xray.ClientTraffic
			if err := db.Where("email = ?", email).First(&got).Error; err != nil {
				t.Fatalf("reload traffic: %v", err)
			}
			if got.Up != 150 || got.Down != 260 {
				t.Fatalf("traffic = up %d down %d, want 150/260 including the final 50/60", got.Up, got.Down)
			}
		})
	}
}

type transitionEmbeddedManager struct {
	removed []int
	started []int
}

func (m *transitionEmbeddedManager) Remove(id int) { m.removed = append(m.removed, id) }
func (m *transitionEmbeddedManager) Reconcile(desired []amneziawgnet.Desired) {
	m.started = m.started[:0]
	for _, d := range desired {
		m.started = append(m.started, d.Instance.Id)
	}
}

type transitionKernelManager struct {
	id          int
	running     bool
	failRemoval bool
}

func (m *transitionKernelManager) Reconcile(desired []amneziawgkern.Desired) {
	wanted := false
	for _, d := range desired {
		if d.Instance.Id == m.id {
			wanted = true
		}
	}
	if wanted {
		m.running = true
		return
	}
	if m.running && m.failRemoval {
		m.failRemoval = false
		return
	}
	m.running = false
}

func (m *transitionKernelManager) Running(id int) bool { return id == m.id && m.running }

func TestKernelToEmbeddedWaitsForKernelTeardownRetry(t *testing.T) {
	const id = 91
	embedded := []amneziawgnet.Desired{{Instance: amneziawg.Instance{Id: id}}}
	kernel := &transitionKernelManager{id: id, running: true, failRemoval: true}
	embeddedManager := &transitionEmbeddedManager{}

	reconcileAmneziaWGDatapaths(embedded, nil, embeddedManager, kernel)
	if len(embeddedManager.started) != 0 {
		t.Fatalf("embedded reconcile started inbound %d while kernel teardown was still pending", embeddedManager.started[0])
	}
	if !kernel.running {
		t.Fatal("first reconcile should retain the kernel interface after its simulated teardown failure")
	}

	reconcileAmneziaWGDatapaths(embedded, nil, embeddedManager, kernel)
	if kernel.running {
		t.Fatal("second reconcile should complete the kernel teardown")
	}
	if len(embeddedManager.started) != 1 || embeddedManager.started[0] != id {
		t.Fatalf("embedded reconcile after teardown = %v, want inbound %d", embeddedManager.started, id)
	}
}
