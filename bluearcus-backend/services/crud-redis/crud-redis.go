package crudredis

import (
	"bluearcus-backend/models"
	"time"
)

type Service interface {
	Get(graphType string, startdate, enddate time.Time) ([]models.DataPoint, []string)
	Set(dataFromDB map[string][]int64, data []models.DataPoint, graphType string)(error)
}

var defaultService Service

func GetService() Service {
	return defaultService
}

func Init(service Service) {
	defaultService = service
}