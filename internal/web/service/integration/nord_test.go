package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type nordRoundTripFunc func(*http.Request) (*http.Response, error)

func (f nordRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func stubNordHTTPClient(t *testing.T, fn nordRoundTripFunc) {
	t.Helper()
	previous := nordHTTPClient
	nordHTTPClient = &http.Client{Transport: fn}
	t.Cleanup(func() { nordHTTPClient = previous })
}

func nordJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestNordCountriesOnlyRequestsNordLynxServerCountries(t *testing.T) {
	stubNordHTTPClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/servers/countries" {
			t.Fatalf("country path = %q", req.URL.Path)
		}
		if got := req.URL.Query().Get("filters[servers_technologies][identifier]"); got != "wireguard_udp" {
			t.Fatalf("NordLynx technology filter = %q", got)
		}
		return nordJSONResponse(`[{"id":228,"name":"United States","code":"US"}]`), nil
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
	stubNordHTTPClient(t, func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Query().Get("filters[country_id]"); got != "225" {
			t.Fatalf("country filter = %q", got)
		}
		if got := req.URL.Query().Get("filters[servers_technologies][identifier]"); got != "wireguard_udp" {
			t.Fatalf("NordLynx technology filter = %q", got)
		}
		return nordJSONResponse(`{"servers":[{"id":1,"load":0},{"id":2,"load":4}]}`), nil
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
