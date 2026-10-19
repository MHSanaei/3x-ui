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

func publishTuicRelayReadiness(cfg *xray.Config) error {
	for i := range cfg.InboundConfigs {
		inbound := &cfg.InboundConfigs[i]
		if !inbound.TuicRelay {
			continue
		}
		gate := tuic.ManagedRelayGate(inbound.Port)
		if gate == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		update, err := gate.BeginUpdate(ctx)
		if err != nil {
			cancel()
			return fmt.Errorf("publish TUIC relay readiness: %w", err)
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
		cancel()
		if err != nil {
			update.Finish(inbound.TuicInboundID, nil, false)
			return fmt.Errorf("TUIC relay not ready: %w", err)
		}
		finishTuicRelayUpdate(update, inbound)
	}
	return nil
}

func repairUnavailableTuicRelays(cfg *xray.Config, apiPort int) error {
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
	if len(repairs) == 0 {
		return nil
	}
	api := &xray.XrayAPI{}
	if err := api.Init(apiPort); err != nil {
		return err
	}
	defer api.Close()
	return applyTuicRelayReplacements(api, repairs)
}

func applyTuicRelayReplacements(api *xray.XrayAPI, changes []xray.InboundReplacement) error {
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
			return fmt.Errorf("pause TUIC relay: %w", err)
		}
		states[port].update = update
	}
	for i, change := range changes {
		if err := replaceTuicRelay(api, change.Previous, change.Next); err != nil {
			var rollbackErr error
			unavailable := make(map[int]bool)
			for j := i; j >= 0; j-- {
				old := changes[j]
				if restoreErr := replaceTuicRelay(api, old.Next, old.Previous); restoreErr != nil {
					rollbackErr = errors.Join(rollbackErr, restoreErr)
					for _, cfg := range []*xray.InboundConfig{old.Previous, old.Next} {
						if cfg != nil {
							unavailable[cfg.Port] = true
						}
					}
				}
			}
			for port, state := range states {
				if state.update == nil {
					continue
				}
				restored := state.previous
				if unavailable[port] {
					restored = nil
				}
				finishTuicRelayUpdate(state.update, restored)
			}
			return errors.Join(fmt.Errorf("TUIC relay update: %w", err), rollbackErr)
		}
	}
	for _, state := range states {
		if state.update != nil {
			finishTuicRelayUpdate(state.update, state.next)
		}
	}
	return nil
}

func replaceTuicRelay(api *xray.XrayAPI, previous, next *xray.InboundConfig) error {
	removed := previous
	if removed == nil {
		removed = next
	}
	if removed == nil {
		return nil
	}
	if err := api.DelInbound(removed.Tag); err != nil && !xray.IsMissingHandlerErr(err) {
		return err
	}
	if next == nil {
		return nil
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if err := api.AddInbound(raw); err != nil {
		return err
	}
	return verifyTuicRelay(previous, next)
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
