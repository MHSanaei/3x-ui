package router_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/common/platform/filesystem"
)

func TestDomainMatcherSerialization(t *testing.T) {

	domains := []*router.Domain{
		{Type: router.Domain_Domain, Value: "google.com"},
		{Type: router.Domain_Domain, Value: "v2ray.com"},
		{Type: router.Domain_Full, Value: "full.example.com"},
	}

	var buf bytes.Buffer
	if err := router.SerializeDomainMatcher(domains, &buf); err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	matcher, err := router.NewDomainMatcherFromBuffer(buf.Bytes())
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	dMatcher := &router.DomainMatcher{
		Matchers: matcher,
	}
	testCases := []struct {
		Input string
		Match bool
	}{
		{"google.com", true},
		{"maps.google.com", true},
		{"v2ray.com", true},
		{"full.example.com", true},

		{"example.com", false},
	}

	for _, tc := range testCases {
		if res := dMatcher.ApplyDomain(tc.Input); res != tc.Match {
			t.Errorf("Match(%s) = %v, want %v", tc.Input, res, tc.Match)
		}
	}
}

func TestGeoSiteSerialization(t *testing.T) {
	sites := []*router.GeoSite{
		{
			CountryCode: "CN",
			Domain: []*router.Domain{
				{Type: router.Domain_Domain, Value: "baidu.cn"},
				{Type: router.Domain_Domain, Value: "qq.com"},
			},
		},
		{
			CountryCode: "US",
			Domain: []*router.Domain{
				{Type: router.Domain_Domain, Value: "google.com"},
				{Type: router.Domain_Domain, Value: "facebook.com"},
			},
		},
	}

	var buf bytes.Buffer
	if err := router.SerializeGeoSiteList(sites, nil, nil, &buf); err != nil {
		t.Fatalf("SerializeGeoSiteList failed: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "matcher.cache")

	f, err := os.Create(path)
	require.NoError(t, err)
	_, err = f.Write(buf.Bytes())
	require.NoError(t, err)
	f.Close()

	f, err = os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	require.NoError(t, err)
	data, _ := filesystem.ReadFile(path)

	// cn
	gp, err := router.LoadGeoSiteMatcher(bytes.NewReader(data), "CN")
	if err != nil {
		t.Fatalf("LoadGeoSiteMatcher(CN) failed: %v", err)
	}

	cnMatcher := &router.DomainMatcher{
		Matchers: gp,
	}

	if !cnMatcher.ApplyDomain("baidu.cn") {
		t.Error("CN matcher should match baidu.cn")
	}
	if cnMatcher.ApplyDomain("google.com") {
		t.Error("CN matcher should NOT match google.com")
	}

	// us
	gp, err = router.LoadGeoSiteMatcher(bytes.NewReader(data), "US")
	if err != nil {
		t.Fatalf("LoadGeoSiteMatcher(US) failed: %v", err)
	}

	usMatcher := &router.DomainMatcher{
		Matchers: gp,
	}
	if !usMatcher.ApplyDomain("google.com") {
		t.Error("US matcher should match google.com")
	}
	if usMatcher.ApplyDomain("baidu.cn") {
		t.Error("US matcher should NOT match baidu.cn")
	}

	// unknown
	_, err = router.LoadGeoSiteMatcher(bytes.NewReader(data), "unknown")
	if err == nil {
		t.Error("LoadGeoSiteMatcher(unknown) should fail")
	}
}
func TestGeoSiteSerializationWithDeps(t *testing.T) {
	sites := []*router.GeoSite{
		{
			CountryCode: "geosite:cn",
			Domain: []*router.Domain{
				{Type: router.Domain_Domain, Value: "baidu.cn"},
			},
		},
		{
			CountryCode: "geosite:google@cn",
			Domain: []*router.Domain{
				{Type: router.Domain_Domain, Value: "google.cn"},
			},
		},
		{
			CountryCode: "rule-1",
			Domain: []*router.Domain{
				{Type: router.Domain_Domain, Value: "google.com"},
			},
		},
	}
	deps := map[string][]string{
		"rule-1": {"geosite:cn", "geosite:google@cn"},
	}

	var buf bytes.Buffer
	err := router.SerializeGeoSiteList(sites, deps, nil, &buf)
	require.NoError(t, err)

	matcher, err := router.LoadGeoSiteMatcher(bytes.NewReader(buf.Bytes()), "rule-1")
	require.NoError(t, err)

	require.True(t, matcher.Match("google.com") != nil)
	require.True(t, matcher.Match("baidu.cn") != nil)
	require.True(t, matcher.Match("google.cn") != nil)
}
