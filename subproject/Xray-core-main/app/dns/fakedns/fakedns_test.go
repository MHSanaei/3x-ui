package fakedns

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/uuid"
	"github.com/xtls/xray-core/features/dns"
	"golang.org/x/sync/errgroup"
)

var ipPrefix = "198.1"

func TestNewFakeDnsHolder(_ *testing.T) {
	_, err := NewFakeDNSHolder()
	common.Must(err)
}

func TestFakeDnsHolderCreateMapping(t *testing.T) {
	fkdns, err := NewFakeDNSHolder()
	common.Must(err)

	addr := fkdns.GetFakeIPForDomain("fakednstest.example.com")
	assert.Equal(t, ipPrefix, addr[0].IP().String()[0:len(ipPrefix)])
}

func TestFakeDnsHolderCreateMappingMany(t *testing.T) {
	fkdns, err := NewFakeDNSHolder()
	common.Must(err)

	addr := fkdns.GetFakeIPForDomain("fakednstest.example.com")
	assert.Equal(t, ipPrefix, addr[0].IP().String()[0:len(ipPrefix)])

	addr2 := fkdns.GetFakeIPForDomain("fakednstest2.example.com")
	assert.Equal(t, ipPrefix, addr2[0].IP().String()[0:len(ipPrefix)])
	assert.NotEqual(t, addr[0].IP().String(), addr2[0].IP().String())
}

func TestFakeDnsHolderCreateMappingManyAndResolve(t *testing.T) {
	fkdns, err := NewFakeDNSHolder()
	common.Must(err)

	addr := fkdns.GetFakeIPForDomain("fakednstest.example.com")
	addr2 := fkdns.GetFakeIPForDomain("fakednstest2.example.com")

	{
		result := fkdns.GetDomainFromFakeDNS(addr[0])
		assert.Equal(t, "fakednstest.example.com", result)
	}

	{
		result := fkdns.GetDomainFromFakeDNS(addr2[0])
		assert.Equal(t, "fakednstest2.example.com", result)
	}
}

func TestFakeDnsHolderCreateMappingManySingleDomain(t *testing.T) {
	fkdns, err := NewFakeDNSHolder()
	common.Must(err)

	addr := fkdns.GetFakeIPForDomain("fakednstest.example.com")
	addr2 := fkdns.GetFakeIPForDomain("fakednstest.example.com")
	assert.Equal(t, addr[0].IP().String(), addr2[0].IP().String())
}

func TestGetFakeIPForDomainConcurrently(t *testing.T) {
	fkdns, err := NewFakeDNSHolder()
	common.Must(err)

	total := 200
	addr := make([][]net.Address, total)
	var errg errgroup.Group
	for i := 0; i < total; i++ {
		errg.Go(testGetFakeIP(i, addr, fkdns))
	}
	errg.Wait()
	for i := 0; i < total; i++ {
		for j := i + 1; j < total; j++ {
			assert.NotEqual(t, addr[i][0].IP().String(), addr[j][0].IP().String())
		}
	}
}

func testGetFakeIP(index int, addr [][]net.Address, fkdns *Holder) func() error {
	return func() error {
		addr[index] = fkdns.GetFakeIPForDomain("fakednstest" + strconv.Itoa(index) + ".example.com")
		return nil
	}
}

func TestFakeDnsHolderCreateMappingAndRollOver(t *testing.T) {
	fkdns, err := NewFakeDNSHolderConfigOnly(&FakeDnsPool{
		IpPool:  dns.FakeIPv4Pool,
		LruSize: 256,
	})
	common.Must(err)

	err = fkdns.Start()

	common.Must(err)

	addr := fkdns.GetFakeIPForDomain("fakednstest.example.com")
	addr2 := fkdns.GetFakeIPForDomain("fakednstest2.example.com")

	for i := 0; i <= 8192; i++ {
		{
			result := fkdns.GetDomainFromFakeDNS(addr[0])
			assert.Equal(t, "fakednstest.example.com", result)
		}

		{
			result := fkdns.GetDomainFromFakeDNS(addr2[0])
			assert.Equal(t, "fakednstest2.example.com", result)
		}

		{
			uuid := uuid.New()
			domain := uuid.String() + ".fakednstest.example.com"
			tempAddr := fkdns.GetFakeIPForDomain(domain)
			rsaddr := tempAddr[0].IP().String()

			result := fkdns.GetDomainFromFakeDNS(net.ParseAddress(rsaddr))
			assert.Equal(t, domain, result)
		}
	}
}

func TestFakeDNSMulti(t *testing.T) {
	fakeMulti, err := NewFakeDNSHolderMulti(&FakeDnsPoolMulti{
		Pools: []*FakeDnsPool{{
			IpPool:  "240.0.0.0/12",
			LruSize: 256,
		}, {
			IpPool:  "fddd:c5b4:ff5f:f4f0::/64",
			LruSize: 256,
		}},
	},
	)
	common.Must(err)

	err = fakeMulti.Start()

	common.Must(err)

	assert.Nil(t, err, "Should not throw error")
	_ = fakeMulti

	t.Run("checkInRange", func(t *testing.T) {
		t.Run("ipv4", func(t *testing.T) {
			inPool := fakeMulti.IsIPInIPPool(net.IPAddress([]byte{240, 0, 0, 5}))
			assert.True(t, inPool)
		})
		t.Run("ipv6", func(t *testing.T) {
			ip, err := net.ResolveIPAddr("ip", "fddd:c5b4:ff5f:f4f0::5")
			assert.Nil(t, err)
			inPool := fakeMulti.IsIPInIPPool(net.IPAddress(ip.IP))
			assert.True(t, inPool)
		})
		t.Run("ipv4_inverse", func(t *testing.T) {
			inPool := fakeMulti.IsIPInIPPool(net.IPAddress([]byte{241, 0, 0, 5}))
			assert.False(t, inPool)
		})
		t.Run("ipv6_inverse", func(t *testing.T) {
			ip, err := net.ResolveIPAddr("ip", "fcdd:c5b4:ff5f:f4f0::5")
			assert.Nil(t, err)
			inPool := fakeMulti.IsIPInIPPool(net.IPAddress(ip.IP))
			assert.False(t, inPool)
		})
	})

	t.Run("allocateTwoAddressForTwoPool", func(t *testing.T) {
		address := fakeMulti.GetFakeIPForDomain("fakednstest.example.com")
		assert.Len(t, address, 2, "should be 2 address one for each pool")
		t.Run("eachOfThemShouldResolve:0", func(t *testing.T) {
			domain := fakeMulti.GetDomainFromFakeDNS(address[0])
			assert.Equal(t, "fakednstest.example.com", domain)
		})
		t.Run("eachOfThemShouldResolve:1", func(t *testing.T) {
			domain := fakeMulti.GetDomainFromFakeDNS(address[1])
			assert.Equal(t, "fakednstest.example.com", domain)
		})
	})

	t.Run("understandIPTypeSelector", func(t *testing.T) {
		t.Run("ipv4", func(t *testing.T) {
			address := fakeMulti.GetFakeIPForDomain3("fakednstestipv4.example.com", true, false)
			assert.Len(t, address, 1, "should be 1 address")
			assert.True(t, address[0].Family().IsIPv4())
		})
		t.Run("ipv6", func(t *testing.T) {
			address := fakeMulti.GetFakeIPForDomain3("fakednstestipv6.example.com", false, true)
			assert.Len(t, address, 1, "should be 1 address")
			assert.True(t, address[0].Family().IsIPv6())
		})
		t.Run("ipv46", func(t *testing.T) {
			address := fakeMulti.GetFakeIPForDomain3("fakednstestipv46.example.com", true, true)
			assert.Len(t, address, 2, "should be 2 address")
			assert.True(t, address[0].Family().IsIPv4())
			assert.True(t, address[1].Family().IsIPv6())
		})
	})
}
