package integration

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/psiphon"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// PsiphonService provides business logic for Psiphon multi-region outbound management.
type PsiphonService struct {
	service.SettingService
}

// GetConfig returns the master Psiphon deployment configuration.
func (s *PsiphonService) GetConfig() (*psiphon.PsiphonConfig, error) {
	cfgJSON, err := s.GetPsiphonConfig()
	if err != nil {
		return nil, err
	}
	return &psiphon.PsiphonConfig{
		ConfigJSON:  cfgJSON,
		BasePort:    psiphon.DefaultBasePort,
		AutoRestart: true,
	}, nil
}

// SaveConfig updates the master deployment configuration in settings and runtime manager.
func (s *PsiphonService) SaveConfig(cfgJSON string) error {
	if err := s.SetPsiphonConfig(cfgJSON); err != nil {
		return err
	}
	psiphon.GetManager().SetBaseConfig(cfgJSON)
	return nil
}

// GetCountries returns available countries merged with current instance state.
func (s *PsiphonService) GetCountries() ([]psiphon.Country, error) {
	db := database.GetDB()
	var instances []*model.PsiphonInstance
	if db != nil {
		_ = db.Find(&instances).Error
	}
	addedMap := make(map[string]bool)
	for _, inst := range instances {
		addedMap[inst.CountryCode] = true
	}

	countries := psiphon.GetManager().GetAvailableCountries()
	for i := range countries {
		if addedMap[countries[i].Code] {
			countries[i].Added = true
		}
	}
	return countries, nil
}

// GetInstances returns all persisted Psiphon instances with live status.
func (s *PsiphonService) GetInstances() ([]*model.PsiphonInstance, error) {
	db := database.GetDB()
	if db == nil {
		return nil, errors.New("database not initialized")
	}
	var instances []*model.PsiphonInstance
	if err := db.Find(&instances).Error; err != nil {
		return nil, err
	}
	mgr := psiphon.GetManager()
	for _, inst := range instances {
		if live, ok := mgr.GetInstance(inst.Id); ok {
			inst.Status = live.Status
			inst.LatencyMs = live.LatencyMs
			if live.ExitIP != "" {
				inst.ExitIP = live.ExitIP
			}
		}
	}
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].CountryCode < instances[j].CountryCode
	})
	return instances, nil
}

// AddCountries creates and starts new Psiphon instances for the given ISO country codes.
func (s *PsiphonService) AddCountries(codes []string) ([]*model.PsiphonInstance, error) {
	db := database.GetDB()
	if db == nil {
		return nil, errors.New("database not initialized")
	}

	targetCodes := codes
	if len(codes) == 1 && strings.EqualFold(codes[0], "ALL") {
		targetCodes = make([]string, 0, len(psiphon.StaticCountries))
		for k := range psiphon.StaticCountries {
			targetCodes = append(targetCodes, k)
		}
		sort.Strings(targetCodes)
	}

	mgr := psiphon.GetManager()
	var created []*model.PsiphonInstance

	for _, raw := range targetCodes {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if code == "" {
			continue
		}
		var existing model.PsiphonInstance
		if err := db.Where("id = ? OR country_code = ?", code, code).First(&existing).Error; err == nil {
			continue
		}

		port, err := mgr.AllocatePort(0)
		if err != nil {
			return created, fmt.Errorf("failed allocating port for %s: %w", code, err)
		}

		inst := &model.PsiphonInstance{
			Id:          code,
			CountryCode: code,
			SocksPort:   port,
			Enabled:     true,
			Status:      psiphon.StatusStopped,
			CreatedAt:   time.Now().UnixMilli(),
		}
		if err := db.Create(inst).Error; err != nil {
			return created, fmt.Errorf("failed saving instance %s: %w", code, err)
		}

		_ = mgr.RegisterInstance(&psiphon.Instance{
			ID:          inst.Id,
			CountryCode: inst.CountryCode,
			CountryName: psiphon.CountryName(inst.CountryCode),
			SocksPort:   inst.SocksPort,
			Status:      psiphon.StatusStopped,
			Enabled:     true,
		})

		if psiphon.HasBinary() {
			if err := mgr.Start(inst.Id); err != nil {
				logger.Warningf("psiphon: start %s on creation failed: %v", inst.Id, err)
			} else {
				inst.Status = psiphon.StatusStarting
				_ = db.Model(inst).Update("status", inst.Status).Error
			}
		}

		created = append(created, inst)
	}

	return created, nil
}

// DeleteInstance stops and removes a country instance and releases its port.
func (s *PsiphonService) DeleteInstance(id string) error {
	db := database.GetDB()
	if db == nil {
		return errors.New("database not initialized")
	}
	psiphon.GetManager().UnregisterInstance(id)
	return db.Where("id = ?", id).Delete(&model.PsiphonInstance{}).Error
}

// ToggleInstance flips the enabled flag and starts or stops the instance.
func (s *PsiphonService) ToggleInstance(id string) (*model.PsiphonInstance, error) {
	db := database.GetDB()
	if db == nil {
		return nil, errors.New("database not initialized")
	}
	var inst model.PsiphonInstance
	if err := db.Where("id = ?", id).First(&inst).Error; err != nil {
		return nil, err
	}

	inst.Enabled = !inst.Enabled
	mgr := psiphon.GetManager()

	if inst.Enabled {
		_ = mgr.RegisterInstance(&psiphon.Instance{
			ID:          inst.Id,
			CountryCode: inst.CountryCode,
			CountryName: psiphon.CountryName(inst.CountryCode),
			SocksPort:   inst.SocksPort,
			Status:      psiphon.StatusStopped,
			Enabled:     true,
		})
		if psiphon.HasBinary() {
			if err := mgr.Start(inst.Id); err != nil {
				inst.Status = psiphon.StatusError
			} else {
				inst.Status = psiphon.StatusStarting
			}
		}
	} else {
		_ = mgr.Stop(inst.Id)
		inst.Status = psiphon.StatusStopped
	}

	if err := db.Save(&inst).Error; err != nil {
		return nil, err
	}
	return &inst, nil
}

// TestLatency runs an active latency and egress probe on a single instance.
func (s *PsiphonService) TestLatency(id string) (*psiphon.ProbeResult, error) {
	db := database.GetDB()
	if db == nil {
		return nil, errors.New("database not initialized")
	}
	var inst model.PsiphonInstance
	if err := db.Where("id = ?", id).First(&inst).Error; err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := psiphon.GetManager().ProbeInstance(ctx, id)
	if err != nil {
		return nil, err
	}

	if res.OK {
		inst.LatencyMs = res.LatencyMs
		if res.ExitIP != "" {
			inst.ExitIP = res.ExitIP
		}
		inst.Status = psiphon.StatusConnected
		_ = db.Model(&inst).Updates(map[string]any{
			"latency_ms": inst.LatencyMs,
			"exit_ip":    inst.ExitIP,
			"status":     inst.Status,
		}).Error
	}
	return res, nil
}

// TestAll runs concurrent latency probes across all enabled instances (max 4 concurrent).
func (s *PsiphonService) TestAll() (map[string]*psiphon.ProbeResult, error) {
	instances, err := s.GetInstances()
	if err != nil {
		return nil, err
	}

	results := make(map[string]*psiphon.ProbeResult)
	var mu sync.Mutex

	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for _, inst := range instances {
		if !inst.Enabled {
			continue
		}
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			res, err := psiphon.GetManager().ProbeInstance(ctx, id)
			mu.Lock()
			if err == nil {
				results[id] = res
			} else {
				results[id] = &psiphon.ProbeResult{Error: err.Error()}
			}
			mu.Unlock()

			if err == nil && res.OK {
				db := database.GetDB()
				if db != nil {
					_ = db.Model(&model.PsiphonInstance{}).Where("id = ?", id).Updates(map[string]any{
						"latency_ms": res.LatencyMs,
						"exit_ip":    res.ExitIP,
						"status":     psiphon.StatusConnected,
					}).Error
				}
			}
		}(inst.Id)
	}

	wg.Wait()
	return results, nil
}

// InitManager loads persisted instances from the database and initializes runtime supervision.
func (s *PsiphonService) InitManager() {
	db := database.GetDB()
	if db == nil {
		return
	}
	if cfgJSON, err := s.GetPsiphonConfig(); err == nil && cfgJSON != "" {
		psiphon.GetManager().SetBaseConfig(cfgJSON)
	}

	var instances []*model.PsiphonInstance
	if err := db.Find(&instances).Error; err != nil {
		return
	}

	mgr := psiphon.GetManager()
	for _, inst := range instances {
		_ = mgr.RegisterInstance(&psiphon.Instance{
			ID:          inst.Id,
			CountryCode: inst.CountryCode,
			CountryName: psiphon.CountryName(inst.CountryCode),
			SocksPort:   inst.SocksPort,
			Status:      psiphon.StatusStopped,
			LatencyMs:   inst.LatencyMs,
			ExitIP:      inst.ExitIP,
			Enabled:     inst.Enabled,
		})
	}

	if psiphon.HasBinary() {
		_ = mgr.StartAll()
	}
}
