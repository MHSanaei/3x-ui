package integration

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/psiphon"
)

func TestPsiphonServiceLifecycle(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))

	svc := &PsiphonService{}
	mgr := psiphon.GetManager()
	mgr.SetDataBaseDir(t.TempDir())

	// 1. Config save & get
	testJSON := `{"PropagationChannelId":"TEST1234"}`
	if err := svc.SaveConfig(testJSON); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}
	cfg, err := svc.GetConfig()
	if err != nil || cfg.ConfigJSON != testJSON {
		t.Fatalf("GetConfig mismatch: got %v, err: %v", cfg, err)
	}

	// 2. Add countries
	created, err := svc.AddCountries([]string{"US", "DE"})
	if err != nil {
		t.Fatalf("AddCountries failed: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(created))
	}
	if created[0].SocksPort == created[1].SocksPort {
		t.Fatalf("ports should be distinct: %d vs %d", created[0].SocksPort, created[1].SocksPort)
	}

	// 3. GetInstances
	instances, err := svc.GetInstances()
	if err != nil {
		t.Fatalf("GetInstances failed: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	// 4. GetCountries
	countries, err := svc.GetCountries()
	if err != nil {
		t.Fatalf("GetCountries failed: %v", err)
	}
	usFound := false
	for _, c := range countries {
		if c.Code == "US" {
			usFound = true
			if !c.Added {
				t.Errorf("expected US to be marked Added")
			}
		}
	}
	if !usFound {
		t.Errorf("expected US country in catalog")
	}

	// 5. ToggleInstance
	toggled, err := svc.ToggleInstance("US")
	if err != nil {
		t.Fatalf("ToggleInstance failed: %v", err)
	}
	if toggled.Enabled {
		t.Errorf("expected US to be disabled after toggle")
	}

	// 6. DeleteInstance
	if err := svc.DeleteInstance("DE"); err != nil {
		t.Fatalf("DeleteInstance failed: %v", err)
	}
	instancesAfter, _ := svc.GetInstances()
	if len(instancesAfter) != 1 {
		t.Fatalf("expected 1 instance after delete, got %d", len(instancesAfter))
	}

	// 7. InitManager
	_ = database.GetDB().Create(&model.PsiphonInstance{
		Id:          "JP",
		CountryCode: "JP",
		SocksPort:   40050,
		Enabled:     true,
		Status:      psiphon.StatusStopped,
	})
	svc.InitManager()
	if _, ok := mgr.GetInstance("JP"); !ok {
		t.Fatalf("expected JP to be registered in manager by InitManager")
	}
}
