package mtproto

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/database/model"
)

func TestParseMetricLine(t *testing.T) {
	name, labels, val, err := parseMetricLine(`mtg_traffic{direction="to_client"} 12345`)
	if err != nil {
		t.Fatal(err)
	}
	if name != "mtg_traffic" {
		t.Fatalf("name=%q", name)
	}
	if labels["direction"] != "to_client" {
		t.Fatalf("labels=%v", labels)
	}
	if val != 12345 {
		t.Fatalf("val=%v", val)
	}

	name2, _, val2, err2 := parseMetricLine(`mtg_concurrency 7`)
	if err2 != nil {
		t.Fatal(err2)
	}
	if name2 != "mtg_concurrency" || val2 != 7 {
		t.Fatalf("got %q %v", name2, val2)
	}
}

func TestInstanceFromInbound(t *testing.T) {
	ib := &model.Inbound{
		Id:       3,
		Tag:      "inbound-3",
		Listen:   "0.0.0.0",
		Port:     8443,
		Protocol: model.MTProto,
		Settings: `{"fakeTlsDomain":"example.com","secret":""}`,
	}
	inst, ok := InstanceFromInbound(ib)
	if !ok {
		t.Fatal("expected a usable instance")
	}
	if inst.Secret == "" {
		t.Fatal("secret should be healed to a non-empty value")
	}
	if inst.Port != 8443 || inst.Id != 3 {
		t.Fatalf("bad instance %+v", inst)
	}

	if _, ok := InstanceFromInbound(&model.Inbound{Protocol: model.VLESS}); ok {
		t.Fatal("non-mtproto inbound should not produce an instance")
	}
}
