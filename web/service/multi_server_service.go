package service

import (
	"x-ui/database"
	"x-ui/database/model"
)

type MultiServerService struct{}

func (s *MultiServerService) GetServers() ([]*model.Server, error) {
	db := database.GetDB()
	var servers []*model.Server
	err := db.Find(&servers).Error
	return servers, err
}

func (s *MultiServerService) GetServer(id int) (*model.Server, error) {
	db := database.GetDB()
	var server model.Server
	err := db.First(&server, id).Error
	return &server, err
}

func (s *MultiServerService) AddServer(server *model.Server) error {
	db := database.GetDB()
	return db.Create(server).Error
}

func (s *MultiServerService) UpdateServer(server *model.Server) error {
	db := database.GetDB()
	return db.Save(server).Error
}

func (s *MultiServerService) DeleteServer(id int) error {
	db := database.GetDB()
	return db.Delete(&model.Server{}, id).Error
}
