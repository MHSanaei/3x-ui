package runtime

import (
	"encoding/json"
	"testing"
)

func TestSnapshotDecodeInboundConfigShapes(t *testing.T) {
	for _, config := range []string{`{"clients":[]}`, `"{\"clients\":[]}"`} {
		raw := `{"structure":{"inbounds":[{"id":7,"tag":"alias","port":443,"protocol":"vless","settings":` + config + `,"streamSettings":` + config + `,"sniffing":` + config + `,"clientStats":[{"email":"ignored","up":999}]}],"attachments":[]}}`
		wire, err := decodeNodeTrafficSnapshot(json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		ib := wire.Structure.Inbounds[0]
		if ib.Id != 7 || ib.Tag != "alias" || ib.Port != 443 || string(ib.Protocol) != "vless" || ib.Settings != `{"clients":[]}` || ib.StreamSettings != ib.Settings || ib.Sniffing != ib.Settings || len(ib.ClientStats) != 0 {
			t.Fatalf("inbound shape changed: %+v", ib)
		}
	}
}
