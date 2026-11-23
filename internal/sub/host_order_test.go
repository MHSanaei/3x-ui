package sub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/testpg"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestSubHosts_Postgres(t *testing.T) {
	if os.Getenv("XUI_DB_TYPE") != "postgres" || os.Getenv("XUI_DB_DSN") == "" {
		t.Skip("requires a disposable PostgreSQL database")
	}
	for _, scenario := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"global", TestSub_GlobalHostOrder},
		{"filters", TestSub_HostOrderFiltersAndFallbacks},
		{"base64", TestSub_HostOrderBase64AndInfoNode},
		{"balancer", TestSub_HostOrderBalancerSlots},
		{"deleted binding", TestSub_ClientHostRule_DeletedHostClosesOnlyItsInbound},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			cleanup, err := testpg.IsolatePackage("host_order")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(cleanup)
			scenario.run(t)
		})
	}
}

func TestSub_HostOrderBalancerSlots(t *testing.T) {
	seedSubDB(t)
	setHostSortMode(t, "hosts-first")
	a := seedSubInbound(t, "order-sub", "a", 46910, 0, wsTLSStream)
	b := seedSubInbound(t, "order-sub", "b", 46911, 10, wsTLSStream)
	seedHost(t, &model.Host{InboundId: a.Id, GroupId: "a", SortOrder: 1, Remark: "A", Address: "a.example.com"})
	seedHost(t, &model.Host{InboundId: a.Id, GroupId: "c", SortOrder: 2, Remark: "C", Address: "c.example.com"})
	seedHost(t, &model.Host{InboundId: b.Id, GroupId: "b", SortOrder: 0, Remark: "B", Address: "b.example.com"})
	bal := seedSubBalancer(t, &model.SubBalancer{Remark: "auto", Strategy: "random", InboundIds: []int{a.Id, b.Id}, SortOrder: 5, Enabled: true})
	body, _, err := NewSubJsonService("", "", "", "", NewSubService("")).GetJson("order-sub", "sub.example.com", true)
	if err != nil {
		t.Fatal(err)
	}
	docs := parseSubJsonDocs(t, body)
	var remarks []string
	for _, doc := range docs {
		if doc["remarks"] == "auto" {
			remarks = append(remarks, "balancer")
			continue
		}
		outbounds := doc["outbounds"].([]any)
		settings := outbounds[0].(map[string]any)["settings"].(map[string]any)
		remarks = append(remarks, settings["address"].(string))
	}
	if !reflect.DeepEqual(remarks, []string{"b.example.com", "a.example.com", "balancer", "c.example.com"}) {
		t.Fatalf("endpoint and balancer slots: %v", remarks)
	}
	prefix := fmt.Sprintf("bal-%d-vless", bal.Id)
	wantTags := []string{prefix, prefix + "-2", prefix + "-3", "direct", "block"}
	if got := docOutboundTags(docs[2]); !reflect.DeepEqual(got, wantTags) {
		t.Fatalf("balancer tags: %v, want %v", got, wantTags)
	}
}

func setHostSortMode(t *testing.T, mode string) {
	t.Helper()
	row := &model.Setting{Key: "subHostSortMode"}
	if err := database.GetDB().Where("key = ?", row.Key).Assign(model.Setting{Value: mode}).FirstOrCreate(row).Error; err != nil {
		t.Fatal(err)
	}
}

func TestSub_HostOrderFiltersAndFallbacks(t *testing.T) {
	seedSubDB(t)
	setHostSortMode(t, "hosts-first")
	ib := seedSubInbound(t, "order-sub", "eligible", 46900, 10, wsTLSStream)
	seedSubInbound(t, "order-sub", "legacy", 46901, -10, `{"network":"ws","security":"tls","wsSettings":{"path":"/"},"tlsSettings":{"serverName":"legacy.sni"},"externalProxy":[{"forceTls":"tls","dest":"legacy.example.com","port":443}]}`)
	noHosts := seedSubInbound(t, "order-sub", "no-hosts", 46902, -20, wsTLSStream)
	if err := database.GetDB().Model(noHosts).Update("listen", "198.51.100.20").Error; err != nil {
		t.Fatal(err)
	}
	foreign := seedSubInbound(t, "other-sub", "foreign", 46903, -40, wsTLSStream)
	excluded := seedSubInbound(t, "order-sub", "excluded", 46904, -30, wsTLSStream)
	if err := database.GetDB().Model(excluded).Update("exclude_from_sub", true).Error; err != nil {
		t.Fatal(err)
	}
	for _, h := range []model.Host{
		{InboundId: ib.Id, GroupId: "main", SortOrder: 0, Address: "a.example.com"},
		{InboundId: ib.Id, GroupId: "off", SortOrder: -5, Address: "off.example.com", IsDisabled: true},
		{InboundId: ib.Id, GroupId: "format", SortOrder: -1, Address: "format.example.com", ExcludeFromSubTypes: []string{"clash"}},
		{InboundId: ib.Id, GroupId: "hidden", SortOrder: 1, Address: "hidden.example.com", IsHidden: true},
		{InboundId: foreign.Id, GroupId: "foreign", SortOrder: -10, Address: "foreign.example.com"},
		{InboundId: excluded.Id, GroupId: "excluded", SortOrder: -20, Address: "excluded.example.com"},
	} {
		row := h
		seedHost(t, &row)
	}
	db := database.GetDB()
	queries := 0
	const callback = "test:6894_hosts"
	if err := db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "hosts" {
			queries++
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callback) })
	for _, format := range []string{"raw", "clash", "json"} {
		queries = 0
		want := []string{"format.example.com", "a.example.com", "hidden.example.com", "198.51.100.20", "legacy.example.com"}
		if format == "clash" {
			want = want[1:]
		}
		if got := renderedHostAddresses(t, format, NewSubService("")); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: %v, want %v", format, got, want)
		}
		if queries != 1 {
			t.Fatalf("%s queried hosts %d times, want 1", format, queries)
		}
	}
}

func TestSub_HostOrderBase64AndInfoNode(t *testing.T) {
	seedSubDB(t)
	setHostSortMode(t, "hosts-first")
	a := seedSubInbound(t, "order-sub", "a", 46905, 0, wsTLSStream)
	b := seedSubInbound(t, "order-sub", "b", 46906, 10, wsTLSStream)
	seedHost(t, &model.Host{InboundId: a.Id, GroupId: "a", SortOrder: 1, Remark: "A", Address: "a.example.com"})
	seedHost(t, &model.Host{InboundId: b.Id, GroupId: "b", SortOrder: 0, Remark: "B", Address: "b.example.com"})
	row := &model.Setting{Key: "subInfoNodeEnable"}
	if err := database.GetDB().Where("key = ?", row.Key).Assign(model.Setting{Value: "true"}).FirstOrCreate(row).Error; err != nil {
		t.Fatal(err)
	}
	router := newSubscriptionTestRouter(subscriptionTestRouterConfig{})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://sub.example.com/sub/order-sub", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("HTTP status %d: %s", response.Code, response.Body.String())
	}
	decoded, err := base64.StdEncoding.DecodeString(response.Body.String())
	if err != nil {
		t.Fatalf("base64 subscription: %v: %s", err, response.Body.String())
	}
	lines := strings.Split(strings.TrimSpace(string(decoded)), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "socks://127.0.0.1") || !strings.Contains(lines[1], "b.example.com") || !strings.Contains(lines[2], "a.example.com") {
		t.Fatalf("encoded order/info node: %q", lines)
	}
}

func renderedHostAddresses(t *testing.T, format string, base *SubService) []string {
	t.Helper()
	var addresses []string
	switch format {
	case "raw":
		links, _, _, _, err := base.GetSubs("order-sub", "sub.example.com")
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range splitLinkLines(strings.Join(links, "\n")) {
			u, err := url.Parse(line)
			if err != nil {
				t.Fatal(err)
			}
			addresses = append(addresses, u.Hostname())
		}
	case "clash":
		body, _, err := NewSubClashService(false, "", false, base).GetClash("order-sub", "sub.example.com")
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Proxies []struct{ Server string } `yaml:"proxies"`
		}
		if err := yaml.Unmarshal([]byte(body), &doc); err != nil {
			t.Fatal(err)
		}
		for _, proxy := range doc.Proxies {
			addresses = append(addresses, proxy.Server)
		}
	case "json":
		body, _, err := NewSubJsonService("", "", "", "", base).GetJson("order-sub", "sub.example.com", true)
		if err != nil {
			t.Fatal(err)
		}
		var docs []struct {
			Outbounds []struct {
				Settings struct {
					Address string `json:"address"`
				} `json:"settings"`
			} `json:"outbounds"`
		}
		if body == "" {
			break
		}
		if err := json.Unmarshal([]byte(body), &docs); err != nil {
			t.Fatal(err)
		}
		for _, doc := range docs {
			if len(doc.Outbounds) == 0 || doc.Outbounds[0].Settings.Address == "" {
				t.Fatalf("JSON proxy shape: %s", body)
			}
			addresses = append(addresses, doc.Outbounds[0].Settings.Address)
		}
	}
	return addresses
}

func TestSub_GlobalHostOrder(t *testing.T) {
	seedSubDB(t)
	first := seedSubInbound(t, "order-sub", "first", 46894, -10, wsTLSStream)
	second := seedSubInbound(t, "order-sub", "second", 46895, 20, wsTLSStream)
	seedSubInbound(t, "order-sub", "fallback", 46896, -20, wsTLSStream)
	for _, host := range []model.Host{
		{GroupId: "a", InboundId: first.Id, SortOrder: 1, Remark: "A", Address: "a.example.com"},
		{GroupId: "b", InboundId: second.Id, SortOrder: 0, Remark: "B", Address: "b.example.com"},
		{GroupId: "c", InboundId: second.Id, SortOrder: 2, Remark: "C", Address: "c.example.com"},
		{GroupId: "shared", InboundId: first.Id, SortOrder: 1, Remark: "Z", Address: "shared1.example.com"},
		{GroupId: "shared", InboundId: first.Id, SortOrder: 1, Remark: "Z", Address: "shared2.example.com"},
		{GroupId: "shared", InboundId: second.Id, SortOrder: 1, Remark: "Z", Address: "shared1.example.com"},
	} {
		h := host
		seedHost(t, &h)
	}
	base := NewSubService("")
	assertAll := func(want []string) {
		t.Helper()
		for _, format := range []string{"raw", "clash", "json"} {
			if got := renderedHostAddresses(t, format, base); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s addresses = %v, want %v", format, got, want)
			}
		}
	}
	assertAll([]string{"203.0.113.5", "a.example.com", "shared1.example.com", "shared2.example.com", "b.example.com", "shared1.example.com", "c.example.com"})
	setHostSortMode(t, "hosts-first")
	assertAll([]string{"b.example.com", "a.example.com", "shared1.example.com", "shared2.example.com", "shared1.example.com", "c.example.com", "203.0.113.5"})
	if err := (&service.HostService{}).ReorderHostGroups([]string{"c", "b", "shared", "a"}); err != nil {
		t.Fatal(err)
	}
	assertAll([]string{"c.example.com", "b.example.com", "shared1.example.com", "shared2.example.com", "shared1.example.com", "a.example.com", "203.0.113.5"})
	setHostSortMode(t, "inbound-first")
	assertAll([]string{"203.0.113.5", "shared1.example.com", "shared2.example.com", "a.example.com", "c.example.com", "b.example.com", "shared1.example.com"})
}

// The hosts-first order renders each host on its own, and a CIDR host must still expand into its
// random addresses there, as it does in inbound order, on every format.
func TestSub_HostOrderExpandsCIDRHosts(t *testing.T) {
	seedSubDB(t)
	setHostSortMode(t, "hosts-first")
	ib := seedSubInbound(t, "order-sub", "cidr", 46910, 0, wsTLSStream)
	seedHost(t, &model.Host{GroupId: "cidr", InboundId: ib.Id, Remark: "R", Address: "198.51.100.0/24", RandomCount: 3})
	_, cidr, _ := net.ParseCIDR("198.51.100.0/24")
	for _, format := range []string{"raw", "clash", "json"} {
		got := renderedHostAddresses(t, format, NewSubService(""))
		if len(got) != 3 {
			t.Fatalf("%s addresses = %v, want 3 random addresses from the range", format, got)
		}
		for _, addr := range got {
			if ip := net.ParseIP(addr); ip == nil || !cidr.Contains(ip) {
				t.Fatalf("%s address %q is not inside 198.51.100.0/24", format, addr)
			}
		}
	}
}

// A client bound to host rules must get only those hosts in the hosts-first order too, on every
// format, and a binding whose hosts are all gone must render nothing rather than every host.
func TestSub_HostOrderRespectsClientHostBindings(t *testing.T) {
	seedSubDB(t)
	setHostSortMode(t, "hosts-first")
	ib := seedSubInbound(t, "order-sub", "bound", 46920, 0, wsTLSStream)
	seedHost(t, &model.Host{GroupId: "a", InboundId: ib.Id, SortOrder: 0, Remark: "A", Address: "a.example.com"})
	b := seedHost(t, &model.Host{GroupId: "b", InboundId: ib.Id, SortOrder: 1, Remark: "B", Address: "b.example.com"})
	bind := func(ids ...int) {
		t.Helper()
		if err := database.GetDB().Model(&model.ClientRecord{}).Where("sub_id = ?", "order-sub").
			Update("client_host_rule_ids", model.EncodeClientHostRuleIds(ids)).Error; err != nil {
			t.Fatal(err)
		}
	}
	bind(b.Id)
	for _, format := range []string{"raw", "clash", "json"} {
		if got := renderedHostAddresses(t, format, NewSubService("")); !reflect.DeepEqual(got, []string{"b.example.com"}) {
			t.Fatalf("%s addresses = %v, want only the bound host", format, got)
		}
	}
	bind(999999)
	for _, format := range []string{"raw", "clash", "json"} {
		if got := renderedHostAddresses(t, format, NewSubService("")); len(got) != 0 {
			t.Fatalf("%s addresses = %v, want none for a binding whose hosts are gone", format, got)
		}
	}
}
