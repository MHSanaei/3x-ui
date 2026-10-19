package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type relayAccount struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

func applyTuicRelayReplacements(api *xray.XrayAPI, changes []xray.InboundReplacement) error {
	for i, change := range changes {
		if err := replaceTuicRelay(api, change.Previous, change.Next); err != nil {
			var rollbackErr error
			for j := i; j >= 0; j-- {
				old := changes[j]
				rollbackErr = errors.Join(rollbackErr, replaceTuicRelay(api, old.Next, old.Previous))
			}
			return errors.Join(fmt.Errorf("TUIC relay update: %w", err), rollbackErr)
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
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
