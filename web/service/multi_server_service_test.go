package service

import (
	"os"
	"testing"
	"x-ui/database"
	"x-ui/database/model"

	"github.com/stretchr/testify/assert"
)

func setup() {
	dbPath := "test.db"
	os.Remove(dbPath)
	database.InitDB(dbPath)
}

func teardown() {
	db, _ := database.GetDB().DB()
	db.Close()
	os.Remove("test.db")
}

func TestMultiServerService(t *testing.T) {
	setup()
	defer teardown()

	service := MultiServerService{}

	// Test AddServer
	server := &model.Server{
		Name:    "test-server",
		Address: "127.0.0.1",
		Port:    54321,
		APIKey:  "test-key",
		Enable:  true,
	}
	err := service.AddServer(server)
	assert.NoError(t, err)

	// Test GetServer
	retrievedServer, err := service.GetServer(server.Id)
	assert.NoError(t, err)
	assert.Equal(t, server.Name, retrievedServer.Name)

	// Test GetServers
	servers, err := service.GetServers()
	assert.NoError(t, err)
	assert.Len(t, servers, 1)

	// Test UpdateServer
	retrievedServer.Name = "updated-server"
	err = service.UpdateServer(retrievedServer)
	assert.NoError(t, err)
	updatedServer, _ := service.GetServer(server.Id)
	assert.Equal(t, "updated-server", updatedServer.Name)

	// Test DeleteServer
	err = service.DeleteServer(server.Id)
	assert.NoError(t, err)
	_, err = service.GetServer(server.Id)
	assert.Error(t, err)
}
