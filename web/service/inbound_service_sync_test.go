package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"x-ui/database"
	"x-ui/database/model"

	"github.com/stretchr/testify/assert"
)

func TestInboundServiceSync(t *testing.T) {
	setup()
	defer teardown()

	// Mock server to simulate a slave
	var receivedApiKey string
	var receivedBody []byte
	mockSlave := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedApiKey = r.Header.Get("Api-Key")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockSlave.Close()

	// Add the mock slave to the database
	multiServerService := MultiServerService{}
	mockSlaveURL, _ := url.Parse(mockSlave.URL)
	mockSlavePort, _ := strconv.Atoi(mockSlaveURL.Port())
	slaveServer := &model.Server{
		Name:    "mock-slave",
		Address: mockSlaveURL.Hostname(),
		Port:    mockSlavePort,
		APIKey:  "slave-api-key",
		Enable:  true,
	}
	multiServerService.AddServer(slaveServer)

	// Create a test inbound and client
	inboundService := InboundService{}
	db := database.GetDB()
	testInbound := &model.Inbound{
		UserId:   1,
		Remark:   "test-inbound",
		Enable:   true,
		Settings: `{"clients":[]}`,
	}
	db.Create(testInbound)

	clientData := model.Client{
		Email: "test@example.com",
		ID:    "test-id",
	}
	clientBytes, _ := json.Marshal([]model.Client{clientData})
	inboundData := &model.Inbound{
		Id:       testInbound.Id,
		Settings: string(clientBytes),
	}

	// Test AddInboundClient sync
	inboundService.AddInboundClient(inboundData)

	assert.Equal(t, "slave-api-key", receivedApiKey)
	var receivedInbound model.Inbound
	json.Unmarshal(receivedBody, &receivedInbound)
	assert.Equal(t, 1, receivedInbound.Id)
}
