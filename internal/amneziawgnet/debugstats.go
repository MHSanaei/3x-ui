package amneziawgnet

import (
	"fmt"
	"net/http"
	"strconv"
)

// RegisterDebugStackStats registers a debug HTTP handler (on
// http.DefaultServeMux, the same mux XUI_PPROF's pprof server listens on)
// exposing a running embedded interface's gVisor TCP stack counters -- the
// only way to see loss/recovery on the tunnel-facing TCP endpoint itself,
// as opposed to the loopback relay socket to Xray, which is all a system
// packet capture (ss/tcpdump) can show. Diagnostic only, gated the same way
// pprof itself is; not meant to be scraped or relied on by anything else.
func RegisterDebugStackStats() {
	http.HandleFunc("/debug/amneziawgstack", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.URL.Query().Get("id"))
		if err != nil {
			http.Error(w, "usage: ?id=<inbound id>", http.StatusBadRequest)
			return
		}
		dev, _, ok := GetManager().Lookup(id)
		if !ok {
			http.Error(w, fmt.Sprintf("no running embedded interface for inbound %d", id), http.StatusNotFound)
			return
		}
		s := dev.Stack.Stats().TCP
		fmt.Fprintf(w, "CurrentEstablished=%d\n", s.CurrentEstablished.Value())
		fmt.Fprintf(w, "Retransmits=%d\n", s.Retransmits.Value())
		fmt.Fprintf(w, "FastRetransmit=%d\n", s.FastRetransmit.Value())
		fmt.Fprintf(w, "SlowStartRetransmits=%d\n", s.SlowStartRetransmits.Value())
		fmt.Fprintf(w, "Timeouts=%d\n", s.Timeouts.Value())
		fmt.Fprintf(w, "FastRecovery=%d\n", s.FastRecovery.Value())
		fmt.Fprintf(w, "SACKRecovery=%d\n", s.SACKRecovery.Value())
		fmt.Fprintf(w, "SpuriousRTORecovery=%d\n", s.SpuriousRTORecovery.Value())
		fmt.Fprintf(w, "EstablishedResets=%d\n", s.EstablishedResets.Value())
		fmt.Fprintf(w, "EstablishedTimedout=%d\n", s.EstablishedTimedout.Value())
	})
}
