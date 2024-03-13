package cruddb

import (
	"bluearcus-backend/inits"
	"bluearcus-backend/models"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

//getData: retrieves data based on the graph type for all available time periods.
func GetData(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	cancel()

	graphType := mux.Vars(r)["graph"]

	// Try to fetch data from Redis cache
	cacheKey := "data:" + graphType
	dbClient := inits.GetDBClient()
	rows, err := dbClient.Query("SELECT x, y FROM graph_data WHERE type = $1", graphType)

	if err != nil {
		log.Printf("Error querying database: %v\n", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Iterate through the rows and construct a slice of DataPoint
	var data []models.DataPoint
	for rows.Next() {
		var dp models.DataPoint
		if err := rows.Scan(&dp.X, &dp.Y); err != nil {
			log.Printf("Error scanning row: %v\n", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		data = append(data, dp)
	}

	// Simulate processing time
	time.Sleep(5000 * time.Millisecond)

	// Store data in Redis cache
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling data: %v\n", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	redisClient := inits.GetRedisClient()
	redisClient.Set(ctx, cacheKey, string(jsonData), 10*time.Minute) // Cache data for 10 minutes

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
