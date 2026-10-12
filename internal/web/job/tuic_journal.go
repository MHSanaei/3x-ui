package job

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
)

var tuicJournalMu sync.Mutex

type tuicTrafficBatch struct {
	ID     string                    `json:"id"`
	Deltas []tuic.ClientTrafficDelta `json:"deltas"`
}

func tuicJournalDir() string { return filepath.Join(config.GetDBFolderPath(), "tuic-traffic-journal") }

func syncJournalDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	file, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func storeTuicBatch(batch tuicTrafficBatch) (string, error) {
	dir := tuicJournalDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := syncJournalDir(filepath.Dir(dir)); err != nil {
		return "", err
	}
	data, err := json.Marshal(batch)
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, ".pending-")
	if err != nil {
		return "", err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if _, err := file.Write(data); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	path := filepath.Join(dir, batch.ID+".json")
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return path, syncJournalDir(dir)
}

func (j *TuicJob) replayTuicJournal() error {
	entries, err := os.ReadDir(tuicJournalDir())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(tuicJournalDir(), entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var batch tuicTrafficBatch
		if err := json.Unmarshal(data, &batch); err != nil {
			return fmt.Errorf("TUIC journal %s: %w", entry.Name(), err)
		}
		if batch.ID+".json" != entry.Name() {
			return fmt.Errorf("TUIC journal batch ID mismatch")
		}
		if err := j.inboundService.AddTuicTrafficBatch(batch.ID, aggregateTuicClientTraffic(batch.Deltas, nil)); err != nil {
			return err
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		if err := syncJournalDir(tuicJournalDir()); err != nil {
			return err
		}
	}
	return nil
}

func (j *TuicJob) flushTuicJournal() error {
	tuicJournalMu.Lock()
	defer tuicJournalMu.Unlock()
	manager := tuic.GetManager()
	_, deltas := manager.CollectAllTraffic()
	if len(deltas) > 0 {
		if path, err := storeTuicBatch(tuicTrafficBatch{ID: uuid.NewString(), Deltas: deltas}); err != nil {
			if path == "" {
				manager.RequeueClientTraffic(deltas)
			}
			return fmt.Errorf("persist TUIC shutdown journal: %w", err)
		}
	}
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = j.replayTuicJournal(); err == nil {
			return nil
		}
		if attempt < 2 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	return fmt.Errorf("TUIC traffic retained in durable journal: %w", err)
}
