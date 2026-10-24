package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type relayAccount struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

type tuicRelayUpdate struct {
	update         *tuic.RelayUpdate
	previous, next *xray.InboundConfig
}

func finishTuicRelayUpdate(update *tuic.RelayUpdate, cfg *xray.InboundConfig) {
	var owner int
	var emails []string
	if cfg != nil {
		owner = cfg.TuicInboundID
		var settings struct {
			Accounts []relayAccount `json:"accounts"`
		}
		if json.Unmarshal(cfg.Settings, &settings) != nil {
			update.Finish(owner, nil, false)
			return
		}
		for _, account := range settings.Accounts {
			emails = append(emails, account.User)
		}
	}
	update.Finish(owner, emails, cfg != nil)
}

func invalidateTuicRelayReadiness(cfg *xray.Config) {
	if cfg == nil {
		return
	}
	for _, inbound := range cfg.InboundConfigs {
		if inbound.TuicRelay {
			if gate := tuic.ManagedRelayGate(inbound.Port); gate != nil {
				gate.Invalidate(inbound.TuicInboundID)
			}
		}
	}
}

func publishTuicRelayReadiness(cfg *xray.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return publishTuicRelayBatch(ctx, cfg)
}

func publishTuicRelayBatch(ctx context.Context, cfg *xray.Config) error {
	invalidateTuicRelayReadiness(cfg)
	failures := make([]error, len(cfg.InboundConfigs))
	slots := make(chan struct{}, 4)
	var workers sync.WaitGroup
	for i := range cfg.InboundConfigs {
		inbound := &cfg.InboundConfigs[i]
		if !inbound.TuicRelay {
			continue
		}
		gate := tuic.ManagedRelayGate(inbound.Port)
		if gate == nil {
			continue
		}
		if err := validateTuicRelay(inbound); err != nil {
			failures[i] = fmt.Errorf("TUIC relay %s not ready: %w", inbound.Tag, err)
			continue
		}
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			failures[i] = ctx.Err()
			continue
		}
		workers.Add(1)
		go func(index int, inbound *xray.InboundConfig, gate *tuic.RelayGate) {
			defer workers.Done()
			defer func() { <-slots }()
			failures[index] = publishTuicRelay(ctx, inbound, gate)
		}(i, inbound, gate)
	}
	workers.Wait()
	err := errors.Join(failures...)
	if err != nil {
		(&XrayService{}).SetToNeedReconcile()
	}
	return err
}

func publishTuicRelay(ctx context.Context, inbound *xray.InboundConfig, gate *tuic.RelayGate) error {
	update, err := gate.BeginUpdate(ctx)
	if err != nil {
		return fmt.Errorf("publish TUIC relay %s readiness: %w", inbound.Tag, err)
	}
	// Starting the child process returns before its listeners have bound.
	// Keep native sessions waiting until the configured accounts really work.
	for {
		err = verifyTuicRelayContext(ctx, nil, inbound)
		if err == nil || ctx.Err() != nil {
			break
		}
		select {
		case <-time.After(20 * time.Millisecond):
		case <-ctx.Done():
		}
	}
	if err != nil {
		update.Finish(inbound.TuicInboundID, nil, false)
		return fmt.Errorf("TUIC relay %s not ready: %w", inbound.Tag, err)
	}
	finishTuicRelayUpdate(update, inbound)
	return nil
}

func unavailableTuicRepairs(cfg *xray.Config) []xray.InboundReplacement {
	var repairs []xray.InboundReplacement
	for i := range cfg.InboundConfigs {
		inbound := &cfg.InboundConfigs[i]
		if !inbound.TuicRelay {
			continue
		}
		gate := tuic.ManagedRelayGate(inbound.Port)
		if gate != nil && !gate.ReadyFor(inbound.TuicInboundID) {
			repairs = append(repairs, xray.InboundReplacement{Previous: inbound, Next: inbound})
		}
	}
	return repairs
}

func repairUnavailableTuicRelays(cfg *xray.Config, apiPort int) error {
	repairs := unavailableTuicRepairs(cfg)
	if len(repairs) == 0 {
		return nil
	}
	api := &xray.XrayAPI{}
	if err := api.Init(apiPort); err != nil {
		(&XrayService{}).SetToNeedReconcile()
		return err
	}
	defer api.Close()
	err := applyTuicRelayReplacements(api, repairs)
	if err != nil {
		(&XrayService{}).SetToNeedReconcile()
	}
	return err
}

func applyTuicRelayReplacements(api *xray.XrayAPI, changes []xray.InboundReplacement) error {
	var failures error
	for _, group := range groupTuicRelayChanges(changes) {
		failures = errors.Join(failures, applyTuicRelayGroup(api, group))
	}
	return failures
}

func validateTuicRelay(inbound *xray.InboundConfig) error {
	var settings struct {
		Auth     string         `json:"auth"`
		Accounts []relayAccount `json:"accounts"`
	}
	if err := json.Unmarshal(inbound.Settings, &settings); err != nil {
		return err
	}
	if inbound.Protocol != "socks" || settings.Auth != "password" || string(inbound.Listen) != `"127.0.0.1"` {
		return errors.New("TUIC relay must require authentication on loopback")
	}
	for _, account := range settings.Accounts {
		if len(account.User) == 0 || len(account.User) > 255 || len(account.Pass) == 0 || len(account.Pass) > 255 {
			return errors.New("TUIC relay credentials exceed SOCKS5 bounds")
		}
	}
	return nil
}

type tuicRelayTransaction struct{ states map[int]*tuicRelayUpdate }

func (t *tuicRelayTransaction) finish() {
	for _, state := range t.states {
		if state.update != nil {
			finishTuicRelayUpdate(state.update, state.next)
		}
	}
}

func (t *tuicRelayTransaction) rollback(api *xray.XrayAPI, changes []xray.InboundReplacement) error {
	reverse := make([]xray.InboundReplacement, 0, len(changes))
	for i := len(changes) - 1; i >= 0; i-- {
		reverse = append(reverse, xray.InboundReplacement{Previous: changes[i].Next, Next: changes[i].Previous})
	}
	err := replaceTuicRelaySet(api, reverse)
	for _, state := range t.states {
		state.next = state.previous
		if err != nil {
			state.next = nil
		}
	}
	return err
}

func applyTuicRelayGroup(api *xray.XrayAPI, changes []xray.InboundReplacement) error {
	transaction, err := stageTuicRelayGroup(api, changes)
	if transaction != nil {
		transaction.finish()
	}
	return err
}

func stageTuicRelayGroup(api *xray.XrayAPI, changes []xray.InboundReplacement) (*tuicRelayTransaction, error) {
	for _, change := range changes {
		if change.Next != nil {
			if err := validateTuicRelay(change.Next); err != nil {
				return nil, fmt.Errorf("TUIC relay %s update: %w", change.Next.Tag, err)
			}
		}
	}
	states := make(map[int]*tuicRelayUpdate)
	for _, change := range changes {
		for _, cfg := range []*xray.InboundConfig{change.Previous, change.Next} {
			if cfg != nil && states[cfg.Port] == nil {
				states[cfg.Port] = &tuicRelayUpdate{}
			}
		}
		if old := change.Previous; old != nil {
			if states[old.Port].previous == nil {
				states[old.Port].previous = old
			}
			states[old.Port].next = nil
		}
	}
	for _, change := range changes {
		if next := change.Next; next != nil {
			states[next.Port].next = next
		}
	}
	ports := make([]int, 0, len(states))
	for port := range states {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, port := range ports {
		gate := tuic.ManagedRelayGate(port)
		if gate == nil {
			continue
		}
		update, err := gate.BeginUpdate(ctx)
		if err != nil {
			for _, state := range states {
				if state.update != nil {
					state.update.Cancel()
				}
			}
			return nil, fmt.Errorf("pause TUIC relay: %w", err)
		}
		states[port].update = update
	}
	transaction := &tuicRelayTransaction{states: states}
	if err := replaceTuicRelaySet(api, changes); err != nil {
		rollbackErr := transaction.rollback(api, changes)
		return transaction, errors.Join(fmt.Errorf("TUIC relay update: %w", err), rollbackErr)
	}
	return transaction, nil
}

func replaceTuicRelaySet(api *xray.XrayAPI, changes []xray.InboundReplacement) error {
	removed := make(map[string]bool)
	for _, change := range changes {
		inbound := change.Previous
		if inbound == nil {
			inbound = change.Next
		}
		if inbound == nil || removed[inbound.Tag] {
			continue
		}
		if err := api.DelInbound(inbound.Tag); err != nil && !xray.IsMissingHandlerErr(err) {
			return err
		}
		removed[inbound.Tag] = true
	}
	for _, change := range changes {
		if change.Next == nil {
			continue
		}
		raw, err := json.Marshal(change.Next)
		if err != nil {
			return err
		}
		if err := api.AddInbound(raw); err != nil {
			return err
		}
		if err := verifyTuicRelay(change.Previous, change.Next); err != nil {
			return err
		}
	}
	return nil
}

func verifyTuicRelay(previous, next *xray.InboundConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return verifyTuicRelayContext(ctx, previous, next)
}

func verifyTuicRelayContext(ctx context.Context, previous, next *xray.InboundConfig) error {
	var settings struct {
		Auth     string         `json:"auth"`
		Accounts []relayAccount `json:"accounts"`
	}
	if err := json.Unmarshal(next.Settings, &settings); err != nil {
		return err
	}
	if settings.Auth != "password" || string(next.Listen) != `"127.0.0.1"` {
		return errors.New("TUIC relay must require authentication on loopback")
	}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(next.Port))
	wanted := make(map[string]string, len(settings.Accounts))
	for _, account := range settings.Accounts {
		wanted[account.User] = account.Pass
	}
	unknown := "_tuic_probe_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	for wanted[unknown] != "" {
		unknown += "_"
	}
	accepted, err := probeRelayAccount(ctx, addr, relayAccount{User: unknown, Pass: "invalid"})
	if err != nil {
		return err
	}
	if accepted {
		return errors.New("TUIC relay accepted an unknown account")
	}
	for _, account := range settings.Accounts {
		accepted, err := probeRelayAccount(ctx, addr, account)
		if err != nil {
			return err
		}
		if !accepted {
			return errors.New("TUIC relay did not install every configured account")
		}
	}
	if previous != nil {
		var old struct {
			Accounts []relayAccount `json:"accounts"`
		}
		if err := json.Unmarshal(previous.Settings, &old); err != nil {
			return err
		}
		for _, account := range old.Accounts {
			// A rejected replacement can contain credentials SOCKS5 cannot encode.
			// Such accounts cannot authenticate and need no removal probe on rollback.
			if len(account.User) == 0 || len(account.User) > 255 || len(account.Pass) == 0 || len(account.Pass) > 255 {
				continue
			}
			if password, exists := wanted[account.User]; exists && password == account.Pass {
				continue
			}
			accepted, err := probeRelayAccount(ctx, addr, account)
			if err != nil {
				return err
			}
			if accepted {
				return errors.New("TUIC relay retained a removed account")
			}
		}
	}
	return nil
}

func probeRelayAccount(ctx context.Context, addr string, account relayAccount) (bool, error) {
	if len(account.User) == 0 || len(account.User) > 255 || len(account.Pass) == 0 || len(account.Pass) > 255 {
		return false, errors.New("TUIC relay credentials exceed SOCKS5 bounds")
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return false, err
	}
	// Probes carry no relayed data; abortive close avoids TIME_WAIT per account.
	_ = conn.(*net.TCPConn).SetLinger(0)
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	if _, err := conn.Write([]byte{5, 1, 2}); err != nil {
		return false, err
	}
	var response [2]byte
	if _, err := io.ReadFull(conn, response[:]); err != nil {
		return false, err
	}
	if response != [2]byte{5, 2} {
		return false, errors.New("TUIC relay did not require password authentication")
	}
	request := append([]byte{1, byte(len(account.User))}, account.User...)
	request = append(request, byte(len(account.Pass)))
	request = append(request, account.Pass...)
	if _, err := conn.Write(request); err != nil {
		return false, err
	}
	if _, err := io.ReadFull(conn, response[:]); err != nil {
		return false, err
	}
	if response[0] != 1 {
		return false, errors.New("TUIC relay returned an invalid authentication response")
	}
	return response[1] == 0, nil
}
