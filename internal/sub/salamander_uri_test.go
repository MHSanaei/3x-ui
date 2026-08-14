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
	if got := extraSalamanderKeys(map[string]any{"password": "pw"}); len(got) != 0 {
		t.Fatalf("expressible settings reported extras: %v", got)
	}
	got := extraSalamanderKeys(map[string]any{"password": "pw", "packetSize": "512-1200"})
	if want := []string{"packetSize"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("extraSalamanderKeys = %v, want %v", got, want)
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
	in := makeInbound(unsupportedID, `{"password":"pw","packetSize":"512-1200"}`)
	(&SubService{}).genHysteriaLink(in, "user")
	(&SubService{}).genHysteriaLink(in, "user")
	if got := countWarnings(unsupportedID); got != 1 {
		t.Fatalf("unsupported-settings warning count = %d, want 1", got)
	}
}
