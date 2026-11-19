package email

import (
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestUserTrafficEmailHasUserMeasurementsAndEscapesRemark(t *testing.T) {
	s := new(Subscriber)
	for _, kind := range []eventbus.EventType{eventbus.EventUserRateHigh, eventbus.EventUserTrafficHigh} {
		subject, body := s.formatMessage(eventbus.Event{Type: kind, Source: "alice", Timestamp: time.Now(), Data: &eventbus.UserTrafficData{Email: "alice", Name: "<script>bad</script>", Bytes: 480_000_000, Threshold: 7_000_000, DurationMS: 60_000}})
		if !strings.Contains(subject, "alice") || !strings.Contains(body, "alice") {
			t.Fatal("missing user")
		}
		if strings.Contains(body, "<script>") || !strings.Contains(body, "&lt;script&gt;") {
			t.Fatal("unescaped remark")
		}
		if kind == eventbus.EventUserRateHigh && (!strings.Contains(body, "8.00 MB/s") || !strings.Contains(body, "7.00 MB/s")) {
			t.Fatal("incorrect rate units")
		}
		if kind == eventbus.EventUserTrafficHigh && !strings.Contains(body, "0.480 GB") {
			t.Fatal("incorrect traffic units")
		}
	}
}

func TestUserTrafficAlertReachesExistingSMTPSubscriber(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "alerts.db"))
	address, lines := startFakeSMTPServer(t)
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	settings := service.SettingService{}
	for _, err := range []error{
		settings.SetSmtpEnable(true), settings.SetSmtpEnabledEvents("user.rate.high"),
		settings.SetSmtpHost(host), settings.SetSmtpPort(port),
		settings.SetSmtpFrom("panel@example.com"), settings.SetSmtpTo("admin@example.com"),
		settings.SetSmtpEncryptionType("none"),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	bus := eventbus.New(0)
	bus.Subscribe("email-traffic-test", NewSubscriber(settings, NewEmailService(settings)).HandleEvent)
	service.SetEventBus(bus)
	t.Cleanup(func() { bus.Stop(); service.SetEventBus(nil) })
	var sampler service.UserTrafficAlertService
	start := time.Unix(1700000000, 0)
	for i := 0; i <= 12; i++ {
		if err := sampler.Observe(start.Add(time.Duration(i)*5*time.Second), nil, []*xray.ClientTraffic{{Email: "alice", Up: 20_000_000, Down: 20_000_000}}); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		recorded := lines()
		message := strings.Join(recorded, "\n")
		if len(recorded) > 0 && recorded[len(recorded)-1] == "." {
			if !strings.Contains(message, "RCPT TO:<admin@example.com>") || !strings.Contains(message, "8.00 MB/s") || !strings.Contains(message, "alice") {
				t.Fatalf("missing recipient or alert measurements: %s", message)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("traffic event did not reach the existing SMTP subscriber")
}
