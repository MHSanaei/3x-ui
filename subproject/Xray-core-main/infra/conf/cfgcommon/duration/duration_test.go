package duration_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/xtls/xray-core/infra/conf/cfgcommon/duration"
)

type testWithDuration struct {
	Duration duration.Duration
}

func TestDurationJSON(t *testing.T) {
	expected := &testWithDuration{
		Duration: duration.Duration(time.Hour),
	}
	data, err := json.Marshal(expected)
	if err != nil {
		t.Error(err)
		return
	}
	actual := &testWithDuration{}
	err = json.Unmarshal(data, &actual)
	if err != nil {
		t.Error(err)
		return
	}
	if actual.Duration != expected.Duration {
		t.Errorf("expected: %s, actual: %s", time.Duration(expected.Duration), time.Duration(actual.Duration))
	}
}
