package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func stubNordAPI(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	previous := nordAPIBase
	server := httptest.NewServer(handler)
	nordAPIBase = server.URL
	t.Cleanup(func() {
		nordAPIBase = previous
		server.Close()
	})
}

func TestNordCountriesOnlyRequestsNordLynxServerCountries(t *testing.T) {
	stubNordAPI(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/servers/countries" {
			t.Errorf("country path = %q", req.URL.Path)
		}
		if got := req.URL.Query().Get("filters[servers_technologies][identifier]"); got != "wireguard_udp" {
			t.Errorf("NordLynx technology filter = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"id":228,"name":"United States","code":"US"}]`)
	})

	got, err := (&NordService{}).GetCountries()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"code":"US"`) {
		t.Fatalf("countries = %s", got)
	}
}

func TestNordServersPreserveLowLoadServers(t *testing.T) {
	stubNordAPI(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v2/servers" {
			t.Errorf("server path = %q", req.URL.Path)
		}
		if got := req.URL.Query().Get("filters[country_id]"); got != "225" {
			t.Errorf("country filter = %q", got)
		}
		if got := req.URL.Query().Get("filters[servers_technologies][identifier]"); got != "wireguard_udp" {
			t.Errorf("NordLynx technology filter = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"servers":[{"id":1,"load":0},{"id":2,"load":4}]}`)
	})

	got, err := (&NordService{}).GetServers("225")
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Servers []struct {
			Load int `json:"load"`
		} `json:"servers"`
	}
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Servers) != 2 || payload.Servers[0].Load != 0 || payload.Servers[1].Load != 4 {
		t.Fatalf("servers = %+v", payload.Servers)
	}
}
