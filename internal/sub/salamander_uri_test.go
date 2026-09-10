package sub

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func TestExtraSalamanderKeys(t *testing.T) {
	if got := extraSalamanderKeys(map[string]any{"password": "pw"}, false); len(got) != 0 {
		t.Fatalf("expressible settings reported extras: %v", got)
	}
	// packetSize exports as the v2rayN gecko fields when expressed; a truly
	// unexpressible key always is. An inexpressible packetSize stays extra.
	in := map[string]any{"password": "pw", "headerType": "dns"}
	if got, want := extraSalamanderKeys(in, false), []string{"headerType"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("extraSalamanderKeys = %v, want %v", got, want)
	}
	full := map[string]any{"password": "pw", "packetSize": "512-1200", "headerType": "dns"}
	if got, want := extraSalamanderKeys(full, true), []string{"headerType"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expressed packetSize not excluded: %v, want %v", got, want)
	}
	if got, want := extraSalamanderKeys(full, false), []string{"headerType", "packetSize"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpressed packetSize not reported: %v, want %v", got, want)
	}
}

func TestGenHysteriaLinkWarnsOnceForUnsupportedSalamanderSettings(t *testing.T) {
	makeInbound := func(id int, settings string) *model.Inbound {
		return &model.Inbound{
			Id: id, Listen: "203.0.113.1", Port: 443, Protocol: model.Hysteria,
			Settings:       `{"version":2,"clients":[{"auth":"secret","email":"user"}]}`,
			StreamSettings: `{"security":"tls","finalmask":{"udp":[{"type":"salamander","settings":` + settings + `}]}}`,
		}
	}
	countWarnings := func(id int) int {
		needle := "inbound " + strconv.Itoa(id) + ": salamander settings"
		count := 0
		for _, line := range logger.GetLogs(100, "warning") {
			if strings.Contains(line, needle) {
				count++
			}
		}
		return count
	}

	const standardID = 910001
	(&SubService{}).genHysteriaLink(makeInbound(standardID, `{"password":"pw"}`), "user")
	if got := countWarnings(standardID); got != 0 {
		t.Fatalf("password-only warning count = %d, want 0", got)
	}

	const unsupportedID = 910002
	in := makeInbound(unsupportedID, `{"password":"pw","headerType":"dns"}`)
	(&SubService{}).genHysteriaLink(in, "user")
	(&SubService{}).genHysteriaLink(in, "user")
	if got := countWarnings(unsupportedID); got != 1 {
		t.Fatalf("unsupported-settings warning count = %d, want 1", got)
	}
}

// A mask with BOTH an expressible packetSize and another key must still warn
// about the leftover key while emitting the gecko URI.
func TestGenHysteriaLinkGeckoStillWarnsOnExtraKeys(t *testing.T) {
	in := &model.Inbound{
		Id: 910003, Listen: "203.0.113.1", Port: 443, Protocol: model.Hysteria,
		Settings:       `{"version":2,"clients":[{"auth":"secret","email":"user"}]}`,
		StreamSettings: `{"security":"tls","finalmask":{"udp":[{"type":"salamander","settings":{"password":"pw","packetSize":"512-1200","headerType":"dns"}}]}}`,
	}
	got := (&SubService{}).genHysteriaLink(in, "user")
	if !strings.Contains(got, "obfs=gecko") {
		t.Fatalf("gecko not emitted for valid packetSize:\n %s", got)
	}
	needle := "inbound 910003: salamander settings"
	found := 0
	for _, line := range logger.GetLogs(100, "warning") {
		if strings.Contains(line, needle) {
			found++
		}
	}
	if found == 0 {
		t.Fatal("leftover salamander key did not warn alongside the gecko export")
	}
}
