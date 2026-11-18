package ha

import (
	"errors"
	"slices"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

func at(sec float64) time.Time { return t0.Add(time.Duration(sec * float64(time.Second))) }

func cluster3() Config {
	return Config{
		Self: "de", Peers: []string{"nl", "pl"}, RequirePatroni: true,
		FenceAfter: 10 * time.Second, MasterHold: 5 * time.Second,
	}
}

// primary is an observation "local PG primary, Patroni leader, peers seen visible".
func primary(sec float64, seen ...string) Sample {
	return Sample{At: at(sec), PatroniLeader: true, Seen: seen}
}

// run feeds samples starting from a state that already saw an observation, so
// master_hold applies from the first sample (unlike a fresh process).
func run(cfg Config, ss ...Sample) []State {
	var st State
	out := make([]State, 0, len(ss))
	for _, s := range ss {
		st = Next(cfg, st, s)
		out = append(out, st)
	}
	return out
}

func TestQuorumSize(t *testing.T) {
	for _, c := range []struct{ peers, want int }{{0, 1}, {1, 2}, {2, 2}, {3, 3}, {4, 3}} {
		cfg := Config{Peers: make([]string, c.peers)}
		if got := cfg.Quorum(); got != c.want {
			t.Fatalf("%d peers: quorum %d, want %d", c.peers, got, c.want)
		}
	}
}

func TestNextMasterOnlyAfterHold(t *testing.T) {
	sts := run(cluster3(), primary(0, "nl"), primary(2, "nl"), primary(4.9, "nl"), primary(5, "nl"))
	for i := range 3 {
		if sts[i].Role != Fenced || !sts[i].Holding() {
			t.Fatalf("step %d: before master_hold want fenced+holding, got %+v", i, sts[i])
		}
	}
	if !sts[3].IsMaster() || sts[3].Master != "de" {
		t.Fatalf("after master_hold want master de, got %+v", sts[3])
	}
}

func TestNextFreshProcessSkipsHold(t *testing.T) {
	st := Next(cluster3(), Initial(), primary(0, "nl"))
	if !st.IsMaster() {
		t.Fatalf("a restarted panel on a healthy primary must be master at once, got %+v", st)
	}
	st = Next(cluster3(), Initial(), primary(0))
	if st.IsMaster() {
		t.Fatalf("a fresh process without quorum must not be master, got %+v", st)
	}
}

func TestNextAsymmetricPartitionKeepsMaster(t *testing.T) {
	sts := run(cluster3(), primary(0, "nl", "pl"), primary(5, "nl", "pl"), primary(7, "nl"), primary(60, "nl"))
	if !sts[3].IsMaster() {
		t.Fatalf("one of two peers visible is still a 2/3 quorum, got %+v", sts[3])
	}
}

func TestNextIsolatedFencedAfterTimeout(t *testing.T) {
	sts := run(cluster3(),
		primary(0, "nl"), primary(5, "nl"),
		primary(6), primary(15),
		primary(15.1),
	)
	if !sts[1].IsMaster() || !sts[3].IsMaster() {
		t.Fatalf("a quorum loss up to fence_after is forgiven: %v %v", sts[1].Role, sts[3].Role)
	}
	if sts[4].Role != Fenced || sts[4].Quorum {
		t.Fatalf("no quorum past fence_after must fence, got %+v", sts[4])
	}
}

func TestNextRegainRequiresHold(t *testing.T) {
	sts := run(cluster3(),
		primary(0, "nl"), primary(5, "nl"),
		primary(20),
		primary(21, "pl"),
		primary(25.9, "pl"),
		primary(26, "pl"),
	)
	if sts[2].Role != Fenced {
		t.Fatalf("want fenced, got %s", sts[2].Role)
	}
	if sts[3].IsMaster() || sts[4].IsMaster() {
		t.Fatal("regained quorum must wait master_hold again")
	}
	if !sts[5].IsMaster() {
		t.Fatalf("after master_hold want master, got %+v", sts[5])
	}
}

func TestNextFailClosed(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name string
		s    Sample
		want Role
	}{
		{"pg down", Sample{At: at(0), PGErr: boom, PatroniLeader: true, Seen: []string{"nl"}}, Unknown},
		{"pg replica", Sample{At: at(0), InRecovery: true, PatroniLeader: true, Seen: []string{"nl"}}, Replica},
		{"patroni down", Sample{At: at(0), PatroniErr: boom, Seen: []string{"nl"}}, Fenced},
		{"patroni not leader", Sample{At: at(0), Seen: []string{"nl"}}, Fenced},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := Next(cluster3(), Initial(), c.s)
			if st.Role != c.want || st.IsMaster() {
				t.Fatalf("got %+v, want role %s", st, c.want)
			}
		})
	}
}

func TestNextIgnoresUnknownPeersAndSelf(t *testing.T) {
	st := Next(cluster3(), Initial(), primary(0, "de", "evil", "evil"))
	if st.IsMaster() || st.Quorum {
		t.Fatalf("self and non-members must not count towards quorum, got %+v", st)
	}
	if len(st.Seen) != 0 {
		t.Fatalf("seen must hold members only, got %v", st.Seen)
	}
}

func TestNextSingleNodeNeedsNoPeers(t *testing.T) {
	cfg := Config{Self: "de", FenceAfter: 10 * time.Second, MasterHold: 5 * time.Second}
	st := Next(cfg, Initial(), Sample{At: at(0)})
	if !st.IsMaster() {
		t.Fatalf("a single member is its own quorum, got %+v", st)
	}
}

func TestNextMasterHintFromPeers(t *testing.T) {
	s := Sample{At: at(0), InRecovery: true, Seen: []string{"nl"}, MasterHint: "nl"}
	st := Next(cluster3(), Initial(), s)
	if st.Master != "nl" || !slices.Equal(st.Seen, []string{"nl"}) {
		t.Fatalf("a replica must pass on the peers' master hint, got %+v", st)
	}
}

func TestNextHoldAfterPatroniFence(t *testing.T) {
	notLeader := Sample{At: at(0), Seen: []string{"nl"}}
	st := Next(cluster3(), Initial(), notLeader)
	st = Next(cluster3(), st, primary(1, "nl"))
	if st.IsMaster() || !st.Holding() {
		t.Fatalf("Patroni just made us leader: serve master_hold first, got %+v", st)
	}
}
