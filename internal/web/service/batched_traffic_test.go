package service

import (
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var testPortCounter uint64 = 50000

// getTestPort returns a unique port using atomic counter + random
func getTestPort(t *testing.T) int {
	return 50000 + int(atomic.AddUint64(&testPortCounter, 1)) + rand.Intn(1000)
}

// TestBatchedTrafficWriterBasic tests basic batched traffic writer functionality
func TestBatchedTrafficWriterBasic(t *testing.T) {
	setupScaleDB(t)
	db := database.GetDB()

	port := getTestPort(t)

	inbound := &model.Inbound{
		Remark:   "test-batched",
		Port:     port,
		Protocol: model.VLESS,
		Settings: clientsSettings(t, []model.Client{
			{Email: "batch@test1", Enable: true, ID: "11111111-1111-1111-1111-111111111111", SubID: "s1"},
			{Email: "batch@test2", Enable: true, ID: "22222222-2222-2222-2222-222222222222", SubID: "s2"},
		}),
		Enable: true,
	}

	inboundSvc := &InboundService{}
	if _, _, err := inboundSvc.AddInbound(inbound); err != nil {
		t.Fatalf("AddInbound: %v", err)
	}

	// Sync clients using ClientService to create client_traffics entries
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(nil, inbound.Id, []model.Client{
		{Email: "batch@test1", Enable: true, ID: "11111111-1111-1111-1111-111111111111", SubID: "s1"},
		{Email: "batch@test2", Enable: true, ID: "22222222-2222-2222-2222-222222222222", SubID: "s2"},
	}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}

	batchWriter := NewBatchedTrafficWriter(inboundSvc, BatchedTrafficConfig{
		FlushInterval: 50 * time.Millisecond,
		MaxBatchSize:  100,
	})

	emails := []string{"batch@test1", "batch@test2"}
	for i := 0; i < 100; i++ {
		var clientTraffics []*xray.ClientTraffic
		for _, email := range emails {
			clientTraffics = append(clientTraffics, &xray.ClientTraffic{
				Email:  email,
				Up:     1024,
				Down:   2048,
				Enable: true,
			})
		}
		batchWriter.Submit(nil, clientTraffics)
	}

	// Close the writer to flush all pending writes
	if err := batchWriter.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var counts []struct {
		Email string
		Up    int64
		Down  int64
	}
	if err := db.Model(&xray.ClientTraffic{}).Where("email IN ?", emails).Select("email, up, down").Scan(&counts).Error; err != nil {
		t.Fatalf("load traffic: %v", err)
	}

	for _, c := range counts {
		expectedUp := int64(100 * 1024)
		expectedDown := int64(100 * 2048)
		if c.Up != expectedUp {
			t.Errorf("%s: up=%d want %d", c.Email, c.Up, expectedUp)
		}
		if c.Down != expectedDown {
			t.Errorf("%s: down=%d want %d", c.Email, c.Down, expectedDown)
		}
	}
}

// TestBatchedTrafficWriteAmplification compares write count with/without batching
func TestBatchedTrafficWriteAmplification(t *testing.T) {
	setupScaleDB(t)
	db := database.GetDB()

	port := getTestPort(t)

	inbound := &model.Inbound{
		Remark:   "test-amplification",
		Port:     port,
		Protocol: model.VLESS,
		Settings: clientsSettings(t, []model.Client{
			{Email: "amp@test", Enable: true, ID: "33333333-3333-3333-3333-333333333333", SubID: "s3"},
		}),
		Enable: true,
	}

	inboundSvc := &InboundService{}
	if _, _, err := inboundSvc.AddInbound(inbound); err != nil {
		t.Fatalf("AddInbound: %v", err)
	}

	// Sync clients using ClientService to create client_traffics entries
	clientSvc := &ClientService{}
	if err := clientSvc.SyncInbound(nil, inbound.Id, []model.Client{
		{Email: "amp@test", Enable: true, ID: "33333333-3333-3333-3333-333333333333", SubID: "s3"},
	}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}

	directWriter := func() {
		clientTraffics := []*xray.ClientTraffic{{Email: "amp@test", Up: 1024, Down: 2048, Enable: true}}
		inboundSvc.AddTraffic(nil, clientTraffics)
	}

	batchWriter := NewBatchedTrafficWriter(inboundSvc, BatchedTrafficConfig{
		FlushInterval: 20 * time.Millisecond,
		MaxBatchSize:  50,
	})

	batchSubmit := func() {
		batchWriter.Submit(nil, []*xray.ClientTraffic{{Email: "amp@test", Up: 1024, Down: 2048, Enable: true}})
	}

	start := time.Now()
	for i := 0; i < 100; i++ {
		directWriter()
	}
	directTime := time.Since(start)

	start = time.Now()
	for i := 0; i < 100; i++ {
		batchSubmit()
	}

	// Close the batch writer to flush all pending writes
	if err := batchWriter.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	batchTime := time.Since(start)

	t.Logf("Direct: %v, Batched: %v", directTime, batchTime)

	var probe xray.ClientTraffic
	if err := db.Where("email = ?", "amp@test").First(&probe).Error; err != nil {
		t.Fatalf("load probe: %v", err)
	}
	expected := int64(200 * 1024)
	if probe.Up != expected {
		t.Errorf("traffic mismatch: up=%d want %d", probe.Up, expected)
	}
}