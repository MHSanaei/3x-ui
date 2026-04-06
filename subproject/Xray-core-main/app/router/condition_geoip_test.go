package router_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/platform"
	"github.com/xtls/xray-core/common/platform/filesystem"
	"google.golang.org/protobuf/proto"
)

func getAssetPath(file string) (string, error) {
	path := platform.GetAssetLocation(file)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		path := filepath.Join("..", "..", "resources", file)
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return "", fmt.Errorf("can't find %s in standard asset locations or {project_root}/resources", file)
		}
		if err != nil {
			return "", fmt.Errorf("can't stat %s: %v", path, err)
		}
		return path, nil
	}
	if err != nil {
		return "", fmt.Errorf("can't stat %s: %v", path, err)
	}

	return path, nil
}

func TestGeoIPMatcher(t *testing.T) {
	cidrList := []*router.CIDR{
		{Ip: []byte{0, 0, 0, 0}, Prefix: 8},
		{Ip: []byte{10, 0, 0, 0}, Prefix: 8},
		{Ip: []byte{100, 64, 0, 0}, Prefix: 10},
		{Ip: []byte{127, 0, 0, 0}, Prefix: 8},
		{Ip: []byte{169, 254, 0, 0}, Prefix: 16},
		{Ip: []byte{172, 16, 0, 0}, Prefix: 12},
		{Ip: []byte{192, 0, 0, 0}, Prefix: 24},
		{Ip: []byte{192, 0, 2, 0}, Prefix: 24},
		{Ip: []byte{192, 168, 0, 0}, Prefix: 16},
		{Ip: []byte{192, 18, 0, 0}, Prefix: 15},
		{Ip: []byte{198, 51, 100, 0}, Prefix: 24},
		{Ip: []byte{203, 0, 113, 0}, Prefix: 24},
		{Ip: []byte{8, 8, 8, 8}, Prefix: 32},
		{Ip: []byte{91, 108, 4, 0}, Prefix: 16},
	}

	matcher, err := router.BuildOptimizedGeoIPMatcher(&router.GeoIP{
		Cidr: cidrList,
	})
	common.Must(err)

	testCases := []struct {
		Input  string
		Output bool
	}{
		{
			Input:  "192.168.1.1",
			Output: true,
		},
		{
			Input:  "192.0.0.0",
			Output: true,
		},
		{
			Input:  "192.0.1.0",
			Output: false,
		},
		{
			Input:  "0.1.0.0",
			Output: true,
		},
		{
			Input:  "1.0.0.1",
			Output: false,
		},
		{
			Input:  "8.8.8.7",
			Output: false,
		},
		{
			Input:  "8.8.8.8",
			Output: true,
		},
		{
			Input:  "2001:cdba::3257:9652",
			Output: false,
		},
		{
			Input:  "91.108.255.254",
			Output: true,
		},
	}

	for _, testCase := range testCases {
		ip := net.ParseAddress(testCase.Input).IP()
		actual := matcher.Match(ip)
		if actual != testCase.Output {
			t.Error("expect input", testCase.Input, "to be", testCase.Output, ", but actually", actual)
		}
	}
}

func TestGeoIPMatcherRegression(t *testing.T) {
	cidrList := []*router.CIDR{
		{Ip: []byte{98, 108, 20, 0}, Prefix: 22},
		{Ip: []byte{98, 108, 20, 0}, Prefix: 23},
	}

	matcher, err := router.BuildOptimizedGeoIPMatcher(&router.GeoIP{
		Cidr: cidrList,
	})
	common.Must(err)

	testCases := []struct {
		Input  string
		Output bool
	}{
		{
			Input:  "98.108.22.11",
			Output: true,
		},
		{
			Input:  "98.108.25.0",
			Output: false,
		},
	}

	for _, testCase := range testCases {
		ip := net.ParseAddress(testCase.Input).IP()
		actual := matcher.Match(ip)
		if actual != testCase.Output {
			t.Error("expect input", testCase.Input, "to be", testCase.Output, ", but actually", actual)
		}
	}
}

func TestGeoIPReverseMatcher(t *testing.T) {
	cidrList := []*router.CIDR{
		{Ip: []byte{8, 8, 8, 8}, Prefix: 32},
		{Ip: []byte{91, 108, 4, 0}, Prefix: 16},
	}
	matcher, err := router.BuildOptimizedGeoIPMatcher(&router.GeoIP{
		Cidr: cidrList,
	})
	common.Must(err)
	matcher.SetReverse(true) // Reverse match

	testCases := []struct {
		Input  string
		Output bool
	}{
		{
			Input:  "8.8.8.8",
			Output: false,
		},
		{
			Input:  "2001:cdba::3257:9652",
			Output: true,
		},
		{
			Input:  "91.108.255.254",
			Output: false,
		},
	}

	for _, testCase := range testCases {
		ip := net.ParseAddress(testCase.Input).IP()
		actual := matcher.Match(ip)
		if actual != testCase.Output {
			t.Error("expect input", testCase.Input, "to be", testCase.Output, ", but actually", actual)
		}
	}
}

func TestGeoIPMatcher4CN(t *testing.T) {
	ips, err := loadGeoIP("CN")
	common.Must(err)

	matcher, err := router.BuildOptimizedGeoIPMatcher(&router.GeoIP{
		Cidr: ips,
	})
	common.Must(err)

	if matcher.Match([]byte{8, 8, 8, 8}) {
		t.Error("expect CN geoip doesn't contain 8.8.8.8, but actually does")
	}
}

func TestGeoIPMatcher6US(t *testing.T) {
	ips, err := loadGeoIP("US")
	common.Must(err)

	matcher, err := router.BuildOptimizedGeoIPMatcher(&router.GeoIP{
		Cidr: ips,
	})
	common.Must(err)

	if !matcher.Match(net.ParseAddress("2001:4860:4860::8888").IP()) {
		t.Error("expect US geoip contain 2001:4860:4860::8888, but actually not")
	}
}

func loadGeoIP(country string) ([]*router.CIDR, error) {
	path, err := getAssetPath("geoip.dat")
	if err != nil {
		return nil, err
	}
	geoipBytes, err := filesystem.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var geoipList router.GeoIPList
	if err := proto.Unmarshal(geoipBytes, &geoipList); err != nil {
		return nil, err
	}

	for _, geoip := range geoipList.Entry {
		if geoip.CountryCode == country {
			return geoip.Cidr, nil
		}
	}

	panic("country not found: " + country)
}

func BenchmarkGeoIPMatcher4CN(b *testing.B) {
	ips, err := loadGeoIP("CN")
	common.Must(err)

	matcher, err := router.BuildOptimizedGeoIPMatcher(&router.GeoIP{
		Cidr: ips,
	})
	common.Must(err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = matcher.Match([]byte{8, 8, 8, 8})
	}
}

func BenchmarkGeoIPMatcher6US(b *testing.B) {
	ips, err := loadGeoIP("US")
	common.Must(err)

	matcher, err := router.BuildOptimizedGeoIPMatcher(&router.GeoIP{
		Cidr: ips,
	})
	common.Must(err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = matcher.Match(net.ParseAddress("2001:4860:4860::8888").IP())
	}
}
