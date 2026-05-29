package job

import "testing"

func TestCheckHashStorageJob_RunWithoutPanicWhenStorageNil(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CheckHashStorageJob.Run panicked when storage is nil: %v", r)
		}
	}()
	NewCheckHashStorageJob().Run()
}
