package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/ha"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// exitDemoted makes systemd (Restart=on-failure) bring the panel back up as a
// standby with no memory of having been master.
const exitDemoted = 3

// haNode is the HA side of the process: role monitor, HA endpoint, gate.
type haNode struct {
	settings ha.Settings
	monitor  *ha.Monitor
	gate     *ha.Gate
}

// startHA starts the role monitor and the HA endpoint; nil when HA is off.
func startHA() (*haNode, error) {
	st, err := ha.SettingsFromEnv()
	if err != nil || !st.Enabled() {
		return nil, err
	}
	if config.GetDBKind() != "postgres" {
		return nil, errors.New("HA mode needs XUI_DB_TYPE=postgres under Patroni")
	}
	pg, err := ha.OpenLocalPG(context.Background(), config.GetDBDSN())
	if err != nil {
		return nil, err
	}
	probes := ha.Probes{PG: pg, Standbys: pg}
	if len(st.Peers) > 0 {
		probes.Peers = &ha.Prober{Peers: st.Peers, Secret: st.Secret}
	}
	if st.PatroniURL != "" {
		pc := ha.NewPatroniClient(st.PatroniURL, nil)
		probes.Patroni = pc
		warnPatroniTimings(pc, st)
	}
	n := &haNode{settings: st, gate: ha.NewGate()}
	n.monitor = ha.NewMonitor(st.RoleConfig(), st.Self, probes, st.Interval)
	ha.SetActive(n.monitor)

	srv := &http.Server{
		Addr:              st.Listen,
		Handler:           ha.EndpointHandler(st.Node, st.Secret, st.Self, n.monitor.Current),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HA endpoint on %s: %v", st.Listen, err)
		}
	}()
	go n.monitor.Run(context.Background(), func(s ha.Snapshot) { n.gate.Observe(s.State) })
	logger.Infof("HA mode: node %s, peers %v, endpoint %s", st.Node, ha.PeerCodes(st.Peers), st.Listen)
	return n, nil
}

func warnPatroniTimings(pc *ha.PatroniClient, st ha.Settings) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dc, err := pc.Config(ctx)
	if err != nil {
		logger.Warning("HA: cannot read Patroni timings to check the fencing invariant:", err)
		return
	}
	// One probe round can take a full interval on top of fence_after.
	for _, p := range dc.Check(st.FenceAfter + 2*st.Interval) {
		logger.Warning("HA:", p)
	}
}

// waitForMaster serves the standby answers until this node becomes master;
// the database is only read here, never migrated or written.
func (n *haNode) waitForMaster() error {
	if err := database.OpenDB(config.GetDBPath()); err != nil {
		return fmt.Errorf("open database read-only: %w", err)
	}
	if !database.GetDB().Migrator().HasTable("settings") {
		// A fresh cluster: the first master migrates; until then only the HA endpoint answers.
		logger.Warning("HA: database not initialized yet, standby without panel and subscription ports")
		return n.awaitPromotion()
	}
	if err := service.CheckHAReady(); err != nil {
		return err
	}
	webStandby := web.NewServer(nil)
	if err := webStandby.StartStandby(ha.StandbyHandler(n.monitor.Current, ha.PanelTarget)); err != nil {
		return fmt.Errorf("standby panel port: %w", err)
	}
	subStandby := sub.NewServer()
	if err := subStandby.StartStandby(ha.StandbyHandler(n.monitor.Current, ha.SubTarget)); err != nil {
		return fmt.Errorf("standby subscription port: %w", err)
	}
	if err := n.awaitPromotion(); err != nil {
		return err
	}
	// Shutdown already closes the listeners; a port still held fails the bind next.
	if err := errors.Join(webStandby.StopStandby(), subStandby.Stop()); err != nil {
		logger.Debug("HA: release standby ports:", err)
	}
	return nil
}

func (n *haNode) awaitPromotion() error {
	logger.Info("HA: standby, waiting to become master")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, os.Interrupt)
	defer signal.Stop(sig)
	select {
	case <-n.gate.Promoted():
	case s := <-sig:
		logger.Info("HA: standby stopped by signal ", s)
		os.Exit(0)
	}
	logger.Warning("HA: this node is master now, starting the panel")
	return nil
}

// demoted is closed once a promoted node loses master; nil (never ready) when HA is off.
func (n *haNode) demoted() <-chan struct{} {
	if n == nil {
		return nil
	}
	return n.gate.Demoted()
}
