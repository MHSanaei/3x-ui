package log_test

import (
	"context"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/xtls/xray-core/app/log"
	"github.com/xtls/xray-core/common"
	clog "github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/testing/mocks"
)

func TestCustomLogHandler(t *testing.T) {
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	var loggedValue []string

	mockHandler := mocks.NewLogHandler(mockCtl)
	mockHandler.EXPECT().Handle(gomock.Any()).AnyTimes().DoAndReturn(func(msg clog.Message) {
		loggedValue = append(loggedValue, msg.String())
	})

	log.RegisterHandlerCreator(log.LogType_Console, func(lt log.LogType, options log.HandlerCreatorOptions) (clog.Handler, error) {
		return mockHandler, nil
	})

	logger, err := log.New(context.Background(), &log.Config{
		ErrorLogLevel: clog.Severity_Debug,
		ErrorLogType:  log.LogType_Console,
		AccessLogType: log.LogType_None,
	})
	common.Must(err)

	common.Must(logger.Start())

	clog.Record(&clog.GeneralMessage{
		Severity: clog.Severity_Debug,
		Content:  "test",
	})

	if len(loggedValue) < 2 {
		t.Fatal("expected 2 log messages, but actually ", loggedValue)
	}

	if loggedValue[1] != "[Debug] test" {
		t.Fatal("expected '[Debug] test', but actually ", loggedValue[1])
	}

	common.Must(logger.Close())
}

func TestMaskAddress(t *testing.T) {
	m4, m6, err := log.ParseMaskAddress("half")
	if err != nil {
		t.Fatal(err)
	}
	maskedAddr := log.MaskedMsgWrapper{
		Mask4: m4,
		Mask6: m6,
	}
	maskedAddr.Message = net.ParseIP("11.45.1.4")
	if maskedAddr.String() != "11.45.*.*" {
		t.Fatal("expected '11.45.*.*', but actually ", maskedAddr.String())
	}
	maskedAddr.Message = net.ParseIP("11:45:14:19:19:81:0::")
	if maskedAddr.String() != "11:45::/32" {
		t.Fatal("expected '11:45::/32', but actually", maskedAddr.String())
	}

	m4, m6, err = log.ParseMaskAddress("/16+/64")
	if err != nil {
		t.Fatal(err)
	}
	maskedAddr = log.MaskedMsgWrapper{
		Mask4: m4,
		Mask6: m6,
	}
	maskedAddr.Message = net.ParseIP("11.45.1.4")
	if maskedAddr.String() != "11.45.*.*" {
		t.Fatal("expected '11.45.*.*', but actually ", maskedAddr.String())
	}
	maskedAddr.Message = net.ParseIP("11:45:14:19:19:81:0::")
	if maskedAddr.String() != "11:45:14:19::/64" {
		t.Fatal("expected '11:45:14:19::/64', but actually", maskedAddr.String())
	}
}
