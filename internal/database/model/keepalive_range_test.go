package model

import (
	"encoding/json"
	"testing"
)

func TestClientKeepAliveRangeJSON(t *testing.T) {
	var client Client
	if err := json.Unmarshal([]byte(`{"keepAlive":"15-25"}`), &client); err != nil {
		t.Fatalf("decode AmneziaWG keepalive range: %v", err)
	}
	got, err := json.Marshal(client)
	if err != nil {
		t.Fatalf("encode AmneziaWG keepalive range: %v", err)
	}
	if string(got) != `{"keepAlive":"15-25"}` {
		t.Fatalf("client JSON = %s, want keepAlive range preserved", got)
	}
}
