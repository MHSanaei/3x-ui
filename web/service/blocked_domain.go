package service

import (
	"x-ui/database"
	"x-ui/database/model"
)

type BlockedDomainService struct{}

func (s *BlockedDomainService) GetAll() ([]model.BlockedDomain, error) {
	db := database.GetDB()
	var domains []model.BlockedDomain
	err := db.Find(&domains).Error
	return domains, err
}

func (s *BlockedDomainService) GetByID(id int) (*model.BlockedDomain, error) {
	db := database.GetDB()
	var domain model.BlockedDomain
	err := db.First(&domain, id).Error
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

func (s *BlockedDomainService) Create(domain *model.BlockedDomain) error {
	db := database.GetDB()
	return db.Create(domain).Error
}

func (s *BlockedDomainService) Update(domain *model.BlockedDomain) error {
	db := database.GetDB()
	return db.Save(domain).Error
}

func (s *BlockedDomainService) Delete(id int) error {
	db := database.GetDB()
	return db.Delete(&model.BlockedDomain{}, id).Error
} 