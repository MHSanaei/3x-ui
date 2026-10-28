package service

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/xtls/xray-core/infra/conf"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func fallbackTestDecryption() string {
	return "mlkem768x25519plus.native.600s." + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
}

func TestVlessFallbackCRUDProtectsCoreConfig(t *testing.T) {
	for _, operation := range []string{"add", "update"} {
		for _, tc := range []struct {
			name, fallback string
			rejected       bool
		}{
			{"empty", `,"fallbacks":[]`, false},
			{"null", `,"fallbacks":null`, false},
			{"absent", "", false},
			{"conflicting", `,"fallbacks":[{"dest":8080}]`, true},
			{"malformed object", `,"fallbacks":{}`, true},
			{"malformed entry", `,"fallbacks":[null]`, true},
		} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				setupConflictDB(t)
				useTestRuntimeManager(t).SetLocalRuntimeOverride(&fakeNodeRuntime{})
				inbound := &model.Inbound{UserId: 1, Port: 32101, Protocol: model.VLESS, Settings: `{"clients":[],"decryption":"none"}`, StreamSettings: `{"network":"tcp","security":"none"}`}
				if operation == "update" {
					if err := database.GetDB().Create(inbound).Error; err != nil {
						t.Fatal(err)
					}
				}
				inbound.Settings = `{"clients":[],"decryption":"` + fallbackTestDecryption() + `","encryption":"link-key","testseed":[1,2,3,4]` + tc.fallback + `}`
				svc := &InboundService{}
				var err error
				if operation == "add" {
					_, _, err = svc.AddInbound(inbound)
				} else {
					_, _, err = svc.UpdateInbound(inbound)
				}
				if tc.rejected {
					if err == nil || !strings.Contains(strings.ToLower(err.Error()), "fallback") {
						t.Fatalf("error = %v, want an actionable fallback validation error", err)
					}
					var rows []model.Inbound
					if err := database.GetDB().Find(&rows).Error; err != nil {
						t.Fatal(err)
					}
					if operation == "add" && len(rows) != 0 || operation == "update" && (len(rows) != 1 || rows[0].Settings != `{"clients":[],"decryption":"none"}`) {
						t.Fatalf("invalid request changed the database: %+v", rows)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				var stored model.Inbound
				if err := database.GetDB().First(&stored, inbound.Id).Error; err != nil {
					t.Fatal(err)
				}
				var core conf.VLessInboundConfig
				if err := json.Unmarshal([]byte(stored.Settings), &core); err != nil {
					t.Fatal(err)
				}
				if _, err := core.Build(); err != nil {
					t.Fatalf("saved configuration rejected by Xray: %v; settings=%s", err, stored.Settings)
				}
				var preserved map[string]json.RawMessage
				_ = json.Unmarshal([]byte(stored.Settings), &preserved)
				var seed []int
				_ = json.Unmarshal(preserved["testseed"], &seed)
				if !reflect.DeepEqual(seed, []int{1, 2, 3, 4}) || core.Decryption != fallbackTestDecryption() {
					t.Fatalf("unrelated VLESS settings changed: %s", stored.Settings)
				}
			})
		}
	}
}

func TestVlessManagedFallbackConflictDoesNotChangeRulesOrInbound(t *testing.T) {
	setupConflictDB(t)
	testVlessManagedFallbackConflict(t)
}

func TestVlessManagedFallbackConflictPostgres(t *testing.T) {
	durablePostgresDB(t)
	testVlessManagedFallbackConflict(t)
}

func testVlessManagedFallbackConflict(t *testing.T) {
	t.Helper()
	useTestRuntimeManager(t).SetLocalRuntimeOverride(&fakeNodeRuntime{})
	db := database.GetDB()
	master := &model.Inbound{UserId: 1, Port: 32102, Protocol: model.VLESS, Settings: `{"clients":[],"decryption":"none"}`, StreamSettings: `{"network":"tcp","security":"tls"}`}
	if err := db.Create(master).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Where("master_id = ?", master.Id).Delete(&model.InboundFallback{}).Error
		_ = db.Delete(master).Error
	})
	original := model.InboundFallback{MasterId: master.Id, Dest: "8080"}
	if err := db.Create(&original).Error; err != nil {
		t.Fatal(err)
	}
	update := *master
	update.Settings = `{"clients":[],"decryption":"` + fallbackTestDecryption() + `"}`
	if _, _, err := (&InboundService{}).UpdateInbound(&update); err == nil || !strings.Contains(strings.ToLower(err.Error()), "fallback") {
		t.Fatalf("UpdateInbound error = %v, want fallback conflict", err)
	}
	var stored model.Inbound
	if err := db.First(&stored, master.Id).Error; err != nil || stored.Settings != master.Settings {
		t.Fatalf("rejected encryption update changed inbound: %s, %v", stored.Settings, err)
	}
	// Stand in for a legacy/imported conflict without going through CRUD.
	if err := db.Model(master).Update("settings", update.Settings).Error; err != nil {
		t.Fatal(err)
	}
	svc := &FallbackService{}
	if err := svc.SetByMaster(master.Id, []FallbackInput{{Dest: "9090"}}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "fallback") {
		t.Fatalf("SetByMaster error = %v, want incompatible master", err)
	}
	rows, err := svc.GetByMaster(master.Id)
	if err != nil || len(rows) != 1 || rows[0].Dest != "8080" || rows[0].Id != original.Id {
		t.Fatalf("rejected replacement changed fallback rules: %+v, %v", rows, err)
	}
	if err := svc.SetByMaster(master.Id, nil); err != nil {
		t.Fatalf("removing conflicting rules must remain possible: %v", err)
	}
	if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
		t.Fatalf("encryption must work after explicit rule removal: %v", err)
	}
}

func TestVlessLegacyFallbacksAreValidatedOnEveryRuntimePath(t *testing.T) {
	for _, managed := range []bool{false, true} {
		for _, nonempty := range []bool{false, true} {
			if managed && !nonempty {
				continue
			}
			for _, path := range []string{"node", "local", "config"} {
				t.Run(path+"/"+map[bool]string{false: "empty", true: "nonempty"}[nonempty]+map[bool]string{false: "raw", true: "managed"}[managed], func(t *testing.T) {
					setupConflictDB(t)
					fb := `[]`
					if nonempty && !managed {
						fb = `[{"dest":8080}]`
					}
					ib := &model.Inbound{UserId: 1, Enable: true, Tag: "legacy", Port: 32103, Protocol: model.VLESS, Settings: `{"clients":[],"decryption":"` + fallbackTestDecryption() + `","fallbacks":` + fb + `}`, StreamSettings: `{"network":"tcp","security":"none"}`}
					db := database.GetDB()
					if err := db.Create(ib).Error; err != nil {
						t.Fatal(err)
					}
					if managed {
						if err := db.Create(&model.InboundFallback{MasterId: ib.Id, Dest: "8080"}).Error; err != nil {
							t.Fatal(err)
						}
					}
					svc := &InboundService{}
					var built *model.Inbound
					var err error
					switch path {
					case "node":
						built, err = svc.buildInboundForNodePush(db, ib)
					case "local":
						built, err = svc.buildInboundForLocalRuntime(db, ib)
					case "config":
						cfg, cfgErr := (&XrayService{}).GetXrayConfig()
						err = cfgErr
						if err == nil {
							for _, inbound := range cfg.InboundConfigs {
								if inbound.Tag == "legacy" {
									built = &model.Inbound{Settings: string(inbound.Settings)}
								}
							}
						}
					}
					if nonempty {
						if err == nil || !strings.Contains(strings.ToLower(err.Error()), "fallback") {
							t.Fatalf("legacy conflict error = %v", err)
						}
						return
					}
					if err != nil || built == nil {
						t.Fatalf("legacy empty repair = %v, %v", built, err)
					}
					var core conf.VLessInboundConfig
					if err := json.Unmarshal([]byte(built.Settings), &core); err != nil {
						t.Fatal(err)
					}
					if _, err := core.Build(); err != nil {
						t.Fatalf("legacy config still crashes Xray: %v", err)
					}
				})
			}
		}
	}
}

func TestVlessFallbackNormalizationPreservesSupportedConfigurations(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol model.Protocol
		settings string
		reject   bool
	}{
		{"plain VLESS", model.VLESS, `{"clients":[],"decryption":"none","fallbacks":[{"dest":8080}]}`, false},
		{"outbound encryption only", model.VLESS, `{"clients":[],"decryption":"none","encryption":"link-only-key","fallbacks":[{"dest":8080}]}`, false},
		{"Trojan", model.Trojan, `{"clients":[],"fallbacks":[{"dest":8080}]}`, false},
		{"malformed plain VLESS", model.VLESS, `{"clients":[],"decryption":"none","fallbacks":[null]}`, true},
		{"invalid fallback destination", model.VLESS, `{"clients":[],"decryption":"none","fallbacks":[{"dest":{}}]}`, true},
		{"invalid fallback path", model.VLESS, `{"clients":[],"decryption":"none","fallbacks":[{"dest":8080,"path":"invalid"}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inbound := &model.Inbound{Protocol: tc.protocol, Settings: tc.settings, StreamSettings: `{"network":"tcp","security":"tls"}`}
			err := normalizeVlessFallbacks(inbound)
			if (err != nil) != tc.reject {
				t.Fatalf("validation error: %v", err)
			}
			if inbound.Settings != tc.settings {
				t.Fatalf("normalizer modified supported/rejected settings: %s", inbound.Settings)
			}
			if tc.reject {
				return
			}
			if !inboundCanHostFallbacks(inbound) {
				t.Fatal("supported master became ineligible")
			}
			var buildErr error
			if tc.protocol == model.VLESS {
				var core conf.VLessInboundConfig
				_ = json.Unmarshal([]byte(inbound.Settings), &core)
				_, buildErr = core.Build()
			} else {
				var core conf.TrojanServerConfig
				_ = json.Unmarshal([]byte(inbound.Settings), &core)
				_, buildErr = core.Build()
			}
			if buildErr != nil {
				t.Fatal(buildErr)
			}
		})
	}
	inbound := &model.Inbound{Protocol: model.VLESS, Settings: `{"decryption":"` + fallbackTestDecryption() + `","fallbacks":[],"unknown":9007199254740993}`}
	if err := normalizeVlessFallbacks(inbound); err != nil {
		t.Fatal(err)
	}
	once := inbound.Settings
	if err := normalizeVlessFallbacks(inbound); err != nil || once != inbound.Settings {
		t.Fatalf("not idempotent: %s; %v", inbound.Settings, err)
	}
	if !strings.Contains(inbound.Settings, "9007199254740993") {
		t.Fatal("normalization rounded an unrelated number")
	}
}

func TestVlessInvalidCandidateKeepsHealthyCore(t *testing.T) {
	setupConflictDB(t)
	process, _ := startBalancerTestXray(t, balancerTestConfig{outbounds: allBalancerTestOutbounds})
	inbound := &model.Inbound{Enable: true, Protocol: model.VLESS, Port: 32119, Settings: `{"clients":[],"decryption":"` + fallbackTestDecryption() + `","fallbacks":[{"dest":8080}]}`, StreamSettings: `{"network":"tcp","security":"none"}`}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatal(err)
	}
	for _, force := range []bool{false, true} {
		err := (&XrayService{}).RestartXray(force)
		if err == nil || !strings.Contains(err.Error(), "fallback") {
			t.Fatalf("restart error: %v", err)
		}
		if currentXrayProcess() != process || !process.IsRunning() {
			t.Fatal("invalid config replaced healthy Xray")
		}
		if _, err := liveBalancerInfo(process, "b1"); err != nil {
			t.Fatalf("healthy core stopped serving its API: %v", err)
		}
		if !strings.Contains((&XrayService{}).GetHeldBackConfig(), "fallback") {
			t.Fatal("refused config is invisible in status")
		}
	}
}

func TestVlessDecryptionAndManagedFallbackWritesSerialize(t *testing.T) {
	setupConflictDB(t)
	useTestRuntimeManager(t).SetLocalRuntimeOverride(&fakeNodeRuntime{})
	master := &model.Inbound{UserId: 1, Protocol: model.VLESS, Port: 32120, Settings: `{"clients":[],"decryption":"none"}`, StreamSettings: `{"network":"tcp","security":"tls"}`}
	if err := database.GetDB().Create(master).Error; err != nil {
		t.Fatal(err)
	}
	update := *master
	update.Settings = `{"clients":[],"decryption":"` + fallbackTestDecryption() + `"}`
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() { <-start; _, _, err := (&InboundService{}).UpdateInbound(&update); results <- err }()
	go func() {
		<-start
		results <- (&FallbackService{}).SetByMaster(master.Id, []FallbackInput{{Dest: "8080"}})
	}()
	close(start)
	first, second := <-results, <-results
	if (first == nil) == (second == nil) {
		t.Fatalf("exactly one conflicting write must succeed: %v; %v", first, second)
	}
	var stored model.Inbound
	if err := database.GetDB().First(&stored, master.Id).Error; err != nil {
		t.Fatal(err)
	}
	if err := validateVlessManagedFallbacks(nil, &stored); err != nil {
		t.Fatalf("concurrent writes persisted incompatible state: %v", err)
	}
}
