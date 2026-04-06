package grpc

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestConfig_GetServiceName(t *testing.T) {
	tests := []struct {
		TestName    string
		ServiceName string
		Expected    string
	}{
		{
			TestName:    "simple no absolute path",
			ServiceName: "hello",
			Expected:    "hello",
		},
		{
			TestName:    "escape no absolute path",
			ServiceName: "hello/world!",
			Expected:    "hello%2Fworld%21",
		},
		{
			TestName:    "absolute path",
			ServiceName: "/my/sample/path/a|b",
			Expected:    "my/sample/path",
		},
		{
			TestName:    "escape absolute path",
			ServiceName: "/hello /world!/a|b",
			Expected:    "hello%20/world%21",
		},
		{
			TestName:    "path with only one '/'",
			ServiceName: "/foo",
			Expected:    "",
		},
	}
	for _, test := range tests {
		t.Run(test.TestName, func(t *testing.T) {
			config := Config{ServiceName: test.ServiceName}
			assert.Equal(t, test.Expected, config.getServiceName())
		})
	}
}

func TestConfig_GetTunStreamName(t *testing.T) {
	tests := []struct {
		TestName    string
		ServiceName string
		Expected    string
	}{
		{
			TestName:    "no absolute path",
			ServiceName: "hello",
			Expected:    "Tun",
		},
		{
			TestName:    "absolute path server",
			ServiceName: "/my/sample/path/tun_service|multi_service",
			Expected:    "tun_service",
		},
		{
			TestName:    "absolute path client",
			ServiceName: "/my/sample/path/tun_service",
			Expected:    "tun_service",
		},
		{
			TestName:    "escape absolute path client",
			ServiceName: "/m y/sa !mple/pa\\th/tun\\_serv!ice",
			Expected:    "tun%5C_serv%21ice",
		},
	}
	for _, test := range tests {
		t.Run(test.TestName, func(t *testing.T) {
			config := Config{ServiceName: test.ServiceName}
			assert.Equal(t, test.Expected, config.getTunStreamName())
		})
	}
}

func TestConfig_GetTunMultiStreamName(t *testing.T) {
	tests := []struct {
		TestName    string
		ServiceName string
		Expected    string
	}{
		{
			TestName:    "no absolute path",
			ServiceName: "hello",
			Expected:    "TunMulti",
		},
		{
			TestName:    "absolute path server",
			ServiceName: "/my/sample/path/tun_service|multi_service",
			Expected:    "multi_service",
		},
		{
			TestName:    "absolute path client",
			ServiceName: "/my/sample/path/multi_service",
			Expected:    "multi_service",
		},
		{
			TestName:    "escape absolute path client",
			ServiceName: "/m y/sa !mple/pa\\th/mu%lti\\_serv!ice",
			Expected:    "mu%25lti%5C_serv%21ice",
		},
	}
	for _, test := range tests {
		t.Run(test.TestName, func(t *testing.T) {
			config := Config{ServiceName: test.ServiceName}
			assert.Equal(t, test.Expected, config.getTunMultiStreamName())
		})
	}
}

func TestSetUserAgent(t *testing.T) {
	ua := "Test/1.0"
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUserAgent(ua))
	assert.NoError(t, err)
	defer conn.Close()
	setUserAgent(conn, ua)
	assert.Equal(t, ua, reflect.ValueOf(conn).Elem().FieldByName("dopts").FieldByName("copts").FieldByName("UserAgent").String())
}
