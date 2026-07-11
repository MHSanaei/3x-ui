package job

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

func banLineCount(t *testing.T, email string) int {
	t.Helper()
	body, err := os.ReadFile(readIpLimitLogPath())
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read 3xipl.log: %v", err)
	}
	return strings.Count(string(body), "[LIMIT_IP] Email = "+email)
}

func TestUpdateInboundClientIps_FrozenLastSeenBannedOnce(t *testing.T) {
	setupIntegrationDB(t)

	const email = "frozen-ban"
	seedInboundWithClient(t, "inbound-frozen-ban", email, 1)

	now := time.Now().Unix()
	deadStart := now - 300
	live := []IPWithTimestamp{
		{IP: "10.2.0.1", Timestamp: deadStart},
		{IP: "192.0.2.7", Timestamp: now},
	}

	j := NewCheckClientIpJob()
	inbound, err := j.getInboundByEmail(email)
	if err != nil {
		t.Fatalf("getInboundByEmail: %v", err)
	}
	row := seedClientIps(t, email, nil)

	if _, banned := j.updateInboundClientIps(database.GetDB(), row, inbound, email, 1, live, true, true); !banned {
		t.Fatalf("first scan: the over-limit stale IP must be banned")
	}
	if got := banLineCount(t, email); got != 1 {
		t.Fatalf("ban lines after first scan = %d, want 1", got)
	}

	if _, banned := j.updateInboundClientIps(database.GetDB(), row, inbound, email, 1, live, true, true); banned {
		t.Fatalf("second scan with a frozen lastSeen must not re-ban a dead connection")
	}
	if got := banLineCount(t, email); got != 1 {
		t.Fatalf("ban lines after frozen rescan = %d, want still 1", got)
	}

	reconnected := []IPWithTimestamp{
		{IP: "10.2.0.1", Timestamp: now + 30},
		{IP: "192.0.2.7", Timestamp: now + 60},
	}
	if _, banned := j.updateInboundClientIps(database.GetDB(), row, inbound, email, 1, reconnected, true, true); !banned {
		t.Fatalf("a reconnect (advanced lastSeen) must be banned again")
	}
	if got := banLineCount(t, email); got != 2 {
		t.Fatalf("ban lines after reconnect = %d, want 2", got)
	}
}
