package maskcompat

import (
	"encoding/json"
	"testing"

	"github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/transport/internet/finalmask/udphop"
)

// coreUDPHop builds a finalmask's UDP masks through conf.Mask, the loader the core
// calls at startup, and returns the udphop config the client would hop with, if any.
func coreUDPHop(t *testing.T, finalmask map[string]any) *udphop.Config {
	t.Helper()
	raw, err := json.Marshal(finalmask)
	if err != nil {
		t.Fatalf("marshal finalmask: %v", err)
	}
	var fm conf.FinalMask
	if err := json.Unmarshal(raw, &fm); err != nil {
		t.Fatalf("decode finalmask: %v", err)
	}
	for _, mask := range fm.Udp {
		built, err := mask.Build(false)
		if err != nil {
			t.Fatalf("the core refuses UDP mask %q: %v", mask.Type, err)
		}
		if hop, ok := built.(*udphop.Config); ok {
			return hop
		}
	}
	return nil
}

func decodeFinalmask(t *testing.T, s string) map[string]any {
	t.Helper()
	var fm map[string]any
	if err := json.Unmarshal([]byte(s), &fm); err != nil {
		t.Fatalf("decode finalmask: %v", err)
	}
	return fm
}

func TestUpgradeLegacyUDPHop(t *testing.T) {
	fm := decodeFinalmask(t, `{"udp":[{"type":"salamander","settings":{"password":"p"}}],
		"quicParams":{"congestion":"bbr","udpHop":{"ports":"20000-20003","interval":"5-10"}}}`)
	if hop := coreUDPHop(t, fm); hop != nil {
		t.Fatal("the core hops on quicParams.udpHop again; the upgrade is no longer needed")
	}

	if !UpgradeLegacyUDPHop(fm) {
		t.Fatal("UpgradeLegacyUDPHop reported no change for a legacy hop")
	}
	got, _ := json.Marshal(fm)
	const want = `{"quicParams":{"congestion":"bbr"},"udp":[{"settings":{"password":"p"},"type":"salamander"},{"settings":{"interval":"5-10","mode":"intervalRemote","remotePorts":"20000-20003"},"type":"udphop"}]}`
	if string(got) != want {
		t.Fatalf("upgraded finalmask\n got: %s\nwant: %s", got, want)
	}
	hop := coreUDPHop(t, fm)
	if hop == nil || !hop.Remote || hop.Local || hop.IntervalMin != 5 || hop.IntervalMax != 10 ||
		len(hop.RemotePorts) != 4 || hop.RemotePorts[0] != 20000 || hop.RemotePorts[3] != 20003 {
		t.Fatalf("the core hops with %+v, want remote ports 20000-20003 every 5-10s", hop)
	}
}

func TestUpgradeLegacyUDPHopEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "an explicit udphop mask wins over the dead key",
			input: `{"udp":[{"type":"udphop","settings":{"mode":"perConnRemote","remotePorts":"443"}}],"quicParams":{"udpHop":{"ports":"20000-30000"}}}`,
			want:  `{"udp":[{"settings":{"mode":"perConnRemote","remotePorts":"443"},"type":"udphop"}]}`,
		},
		{
			name:  "a hop without ports only loses the dead key",
			input: `{"quicParams":{"udpHop":{"interval":"5-10"}}}`,
			want:  `{}`,
		},
		{
			name:  "no interval keeps the core's 30s default",
			input: `{"quicParams":{"udpHop":{"ports":"443,8443"}}}`,
			want:  `{"udp":[{"settings":{"mode":"intervalRemote","remotePorts":"443,8443"},"type":"udphop"}]}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fm := decodeFinalmask(t, tc.input)
			if !UpgradeLegacyUDPHop(fm) {
				t.Fatal("UpgradeLegacyUDPHop reported no change")
			}
			got, _ := json.Marshal(fm)
			if string(got) != tc.want {
				t.Fatalf("upgraded finalmask\n got: %s\nwant: %s", got, tc.want)
			}
			coreUDPHop(t, fm)
		})
	}

	current := decodeFinalmask(t, `{"udp":[{"type":"udphop","settings":{"mode":"intervalRemote","remotePorts":"443"}}],"quicParams":{"congestion":"bbr"}}`)
	if UpgradeLegacyUDPHop(current) {
		t.Fatal("UpgradeLegacyUDPHop rewrote a finalmask without the legacy key")
	}
}
