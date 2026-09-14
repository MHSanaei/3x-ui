package service

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestWeeklyRenewalSearchFailsClosed(t *testing.T) {
	// Keep a background clock reader so -race catches global timezone writes.
	started, stop, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		_ = time.Now()
		close(started)
		for {
			select {
			case <-stop:
				return
			default:
				_ = time.Now()
			}
		}
	}()
	<-started
	t.Cleanup(func() { close(stop); <-done })

	// Fault injection: a valid TZif with twelve absent Sundays, not a claim
	// about real IANA zones. Each following Monday repeats, so no Sunday returns.
	const transitions = 24
	data := make([]byte, 44+transitions*5+2*6+2)
	copy(data, "TZif")
	binary.BigEndian.PutUint32(data[32:36], transitions)
	binary.BigEndian.PutUint32(data[36:40], 2)
	binary.BigEndian.PutUint32(data[40:44], 2)
	from := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	for week := range 12 {
		at := from.AddDate(0, 0, (week+1)*7).Unix()
		for day := range 2 {
			index := week*2 + day
			binary.BigEndian.PutUint32(data[44+index*4:48+index*4], uint32(at+int64(day)*86400))
			data[44+transitions*4+index] = byte(1 - day)
		}
	}
	binary.BigEndian.PutUint32(data[44+transitions*5+6:48+transitions*5+6], 86400)
	data[len(data)-2] = 'X'
	loc, err := time.LoadLocationFromTZData("Test/SkippedSundays", data)
	if err != nil {
		t.Fatal(err)
	}
	if got := nextWeeklyRenewal(from, 7, loc); !got.Equal(from) {
		t.Fatalf("exhausted weekly search advanced to %s, want unchanged %s", got.Format(time.RFC3339), from.Format(time.RFC3339))
	}
	traffic := &xray.ClientTraffic{ExpiryTime: from.UnixMilli(), ResetWeekday: 7, ResetMax: 4, ResetCount: 2, Enable: false, Up: 111, Down: 222}
	before := *traffic
	expiry, renewals := catchUpClientRenewal(traffic, from.AddDate(0, 0, 28).UnixMilli(), loc)
	if expiry != before.ExpiryTime || renewals != 0 || *traffic != before {
		t.Fatalf("exhausted search changed billing/traffic: expiry=%d renewals=%d traffic=%+v", expiry, renewals, traffic)
	}
	for _, tt := range []struct {
		name string
		now  int64
	}{
		{"initial suggestion exhausted", from.UnixMilli()},
		{"catch-up exhausted with allowances left", from.AddDate(0, 3, 0).UnixMilli()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			preview, err := previewClientRenewal(ClientRenewalPreviewRequest{
				ExpiryTime: before.ExpiryTime, ResetWeekday: 7, ResetMax: 4, ResetCount: 2,
			}, tt.now, loc)
			if err == nil || err.Error() != "calendar renewal could not find a future expiry\n" || preview != nil {
				t.Fatalf("failed search preview/error = %+v/%v, want nil/calendar search error", preview, err)
			}
		})
	}
}
