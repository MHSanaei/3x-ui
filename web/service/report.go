package service

import (
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

type ReportService interface {
	SaveReport(report *model.ConnectionReport) error
}

type reportService struct {
}

func NewReportService() ReportService {
	return &reportService{}
}

func (s *reportService) SaveReport(report *model.ConnectionReport) error {
	db := database.GetDB()
	return db.Save(report).Error
}
