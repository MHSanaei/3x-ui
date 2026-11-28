package controller

import (
	"errors"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestAttachJobStorePublishesStatusAndPreventsOverlap(t *testing.T) {
	var store attachJobStore
	block := make(chan struct{})
	id, err := store.start(func() (*service.AttachSelectionResult, error) {
		<-block
		return &service.AttachSelectionResult{Attached: 9997, Skipped: 1, Errors: 2}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if status, ok := store.get(id); !ok || !status.Running {
		t.Fatalf("accepted job status = %#v, found=%t", status, ok)
	}
	if _, err := store.start(nil); err == nil {
		t.Fatal("overlapping job was accepted")
	}
	close(block)

	wait := func(id string) AttachJobStatus {
		t.Helper()
		for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
			if status, ok := store.get(id); ok && !status.Running {
				return status
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("job did not finish")
		return AttachJobStatus{}
	}
	result := wait(id).Result
	if result == nil || result.Attached != 9997 || result.Skipped != 1 || result.Errors != 2 {
		t.Fatalf("partial result lost: %#v", result)
	}
	id, err = store.start(func() (*service.AttachSelectionResult, error) { return nil, errors.New("stale selection") })
	if err != nil {
		t.Fatal(err)
	}
	if status := wait(id); status.Result != nil || status.Error != "stale selection" {
		t.Fatalf("failed result lost: %#v", status)
	}
}
