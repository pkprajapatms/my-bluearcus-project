package redisService

import (
	"bluearcus-backend/inits"
	"bluearcus-backend/models"
	"bluearcus-backend/constants"
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

type Service struct {
	Client *redis.Client
}

func (s Service) Get(graphType string, startdate, enddate time.Time) ([]models.DataPoint, []string) {
	var tempdata models.DataPoint

	// WaitGroup to wait for goroutines to finish
	var wg sync.WaitGroup
	var dataMutex sync.Mutex
	// Iterate through all dates between startdate and enddate and check if the corresponding x and y values are present in Redis
	data := make([]models.DataPoint, 0)
	dateNotFoundInRedis := make([]string, 0)

	redisClient := inits.GetRedisClient()
	for current := startdate; current.Before(enddate); current = current.AddDate(0, 0, 1) {
		cacheKey := graphType + current.Format(constants.DateLayout)
		wg.Add(1)
		go func(cacheKey string, current time.Time) {
			defer wg.Done()
			val, err := redisClient.Get(context.Background(), cacheKey).Result()
			if err == nil {
				// Value found in Redis, parse and add it to data slice
				err = json.Unmarshal([]byte(val), &tempdata)
				if err == nil {
					dataMutex.Lock()
					data = append(data, models.DataPoint{X:tempdata.X, Y:tempdata.Y})
					dataMutex.Unlock()
				}
			} else if err == redis.Nil {
				// Value not found in Redis, add date to list of dates not found
				dataMutex.Lock()
				dateNotFoundInRedis = append(dateNotFoundInRedis, current.Format(constants.DateLayout))
				dataMutex.Unlock()
			} else {
				// Error occurred while fetching from Redis
				log.Println("Error fetching data from Redis:", err)
			}
		}(cacheKey,current)
	}
	wg.Wait()
	return data, dateNotFoundInRedis
}

const (
)

func (s Service) Set(dataFromDB map[string][]int64, data []models.DataPoint, graphType string) error {

	var wg sync.WaitGroup
	// Adding data fetched from the database to Redis
	redisClient := inits.GetRedisClient()
	if redisClient == nil{
		log.Println("Redis not initialized")
		return errors.New("Redis not initialized")
	}
	for key, val := range dataFromDB {
		t, err := time.Parse(constants.DbDatelayout, key)
		if err != nil {
			log.Println("[Redis] data parsing failed")
			continue
		}
		cacheKey := graphType + t.Format(constants.DateLayout)
		wg.Add(1)

		go func(cacheKey string) {
			defer wg.Done()
			jsondata, err := json.Marshal(models.DataPoint{X: val[0], Y: val[1]})
			if err != nil {
				log.Println("[Redis] data parsing failed")
				return
			}
			data = append(data, models.DataPoint{X: val[0], Y: val[1]})
			redisClient.Set(context.Background(), cacheKey, string(jsondata), 60*time.Minute) // Cache data for 10 minutes
		}(cacheKey)
	}
	wg.Wait()
	return nil
}

