package job

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	"github.com/xtls/xray-core/app/proxyman/command"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// A reverse client has to be left alone: the cycle drops its reverse outbound
// handler and re-adds an account without a reverse, which no retry restores.
func TestDisconnectClientTemporarilySkipsReverseClient(t *testing.T) {
	setupIntegrationDB(t)

	const email = "rev-limit-probe"
	inbound := &model.Inbound{
		Id:       1,
		Protocol: model.VLESS,
		Tag:      "rev-limit-probe-tag",
		Settings: `{"clients":[]}`,
	}
	clients := []model.Client{{Email: email, ID: "11111111-1111-1111-1111-111111111111", Reverse: &model.ClientReverse{Tag: "rev-out"}}}

	(&CheckClientIpJob{}).disconnectClientTemporarily(inbound, email, clients)

	var skipped, attempted bool
	for _, line := range logger.GetLogs(500, "warning") {
		if strings.Contains(line, "Not disconnecting "+email) {
			skipped = true
		}
		if strings.Contains(line, "Failed to remove user "+email) {
			attempted = true
		}
	}
	if attempted {
		t.Fatal("a reverse client must not be removed and re-added")
	}
	if !skipped {
		t.Fatal("the skip must be reported, not silent")
	}
}

// The protocol gate must let hysteria through: XrayAPI supports it, and the
// skip left over-limit Hysteria2 sessions alive until the fail2ban ban caught up.
func TestDisconnectClientTemporarilyAllowsHysteria(t *testing.T) {
	setupIntegrationDB(t)

	const email = "hy2-limit-probe"
	inbound := &model.Inbound{
		Id:       1,
		Protocol: model.Hysteria,
		Tag:      "hy2-limit-probe-tag",
		Settings: `{"clients":[]}`,
	}
	clients := []model.Client{{Email: email, Auth: "secret"}}

	(&CheckClientIpJob{}).disconnectClientTemporarily(inbound, email, clients)

	var unsupported, attempted bool
	for _, line := range logger.GetLogs(500, "warning") {
		if strings.Contains(line, "Temporary disconnect is not supported for protocol hysteria") {
			unsupported = true
		}
		if strings.Contains(line, "Failed to remove user "+email) {
			attempted = true
		}
	}
	if unsupported {
		t.Fatal("hysteria was rejected by the protocol gate")
	}
	if !attempted {
		t.Fatal("expected a remove attempt against the Xray API for hysteria")
	}
}

// fakeCoreAPI stands in for the core's HandlerService: it records the user
// operations it is asked for and runs onOp inside each, where another job would act.
type fakeCoreAPI struct {
	command.UnimplementedHandlerServiceServer
	mu   sync.Mutex
	ops  []string
	onOp func(op string)
}

func (f *fakeCoreAPI) AlterInbound(_ context.Context, req *command.AlterInboundRequest) (*command.AlterInboundResponse, error) {
	operation, err := req.Operation.GetInstance()
	if err != nil {
		return nil, err
	}
	op := "add"
	if _, ok := operation.(*command.RemoveUserOperation); ok {
		op = "remove"
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ops = append(f.ops, op)
	f.onOp(op)
	return &command.AlterInboundResponse{}, nil
}

func startFakeCoreAPI(t *testing.T, core *fakeCoreAPI) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	command.RegisterHandlerServiceServer(server, core)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	binDir := t.TempDir()
	t.Setenv("XUI_BIN_FOLDER", binDir)
	config := fmt.Sprintf(`{"inbounds":[{"tag":"api","port":%d}]}`, listener.Addr().(*net.TCPAddr).Port)
	if err := os.WriteFile(filepath.Join(binDir, "config.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The re-add hands the core the scan's snapshot. Whatever took the client out of
// service since then has already been to the core, so the account must not stay.
func TestDisconnectClientTemporarilyDoesNotRestoreDroppedClient(t *testing.T) {
	const email = "readd-probe"
	trafficJobDisables := func(db *gorm.DB, _ *model.Inbound) error {
		return db.Model(&xray.ClientTraffic{}).Where("email = ?", email).Update("enable", false).Error
	}
	tests := []struct {
		name    string
		during  string
		drop    func(db *gorm.DB, inbound *model.Inbound) error
		wantOps []string
	}{
		{name: "still served", wantOps: []string{"remove", "add"}},
		{name: "expired while removed", during: "remove", drop: trafficJobDisables, wantOps: []string{"remove"}},
		{name: "expired while being re-added", during: "add", drop: trafficJobDisables, wantOps: []string{"remove", "add", "remove"}},
		{name: "disabled by the operator", during: "remove", wantOps: []string{"remove"}, drop: func(db *gorm.DB, _ *model.Inbound) error {
			return db.Model(&model.ClientRecord{}).Where("email = ?", email).Update("enable", false).Error
		}},
		{name: "detached from the inbound", during: "remove", wantOps: []string{"remove"}, drop: func(db *gorm.DB, inbound *model.Inbound) error {
			return db.Where("inbound_id = ?", inbound.Id).Delete(&model.ClientInbound{}).Error
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupIntegrationDB(t)
			db := database.GetDB()
			inbound := seedLinkedInboundWithClient(t, "readd-probe-tag", email, 1)
			if err := db.Model(&model.ClientRecord{}).Where("email = ?", email).Update("enable", true).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&xray.ClientTraffic{InboundId: inbound.Id, Email: email, Enable: true}).Error; err != nil {
				t.Fatal(err)
			}

			core := &fakeCoreAPI{}
			core.onOp = func(op string) {
				if op == tt.during {
					if err := tt.drop(db, inbound); err != nil {
						t.Error(err)
					}
				}
			}
			startFakeCoreAPI(t, core)

			clients := []model.Client{{Email: email, ID: "11111111-1111-1111-1111-111111111111", Enable: true}}
			(&CheckClientIpJob{}).disconnectClientTemporarily(inbound, email, clients)

			core.mu.Lock()
			defer core.mu.Unlock()
			if !slices.Equal(core.ops, tt.wantOps) {
				t.Fatalf("core saw %v, want %v", core.ops, tt.wantOps)
			}
		})
	}
}
