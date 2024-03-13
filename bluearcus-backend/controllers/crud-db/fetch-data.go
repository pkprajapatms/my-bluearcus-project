package cruddb

import (
	"bluearcus-backend/constants"
	"bluearcus-backend/inits"
	"bluearcus-backend/models"
	crudredis "bluearcus-backend/services/crud-redis"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/lib/pq"
)

// getDataInRange handles range queries on the dataset.
// It retrieves data points within the specified date range for a given graph type.
// The date range is specified using the "start" and "end" query parameters.
// The response contains a JSON array of DataPoint objects.
func GetDataInRange(w http.ResponseWriter, r *http.Request){

	const (
		dbDatelayout string = "2006-01-02 00:00:00"
		dateLayout string = "2006-01-02"
	)

	graphType := r.URL.Query().Get("type")
	if graphType != "line" && graphType != "bar"{
        http.Error(w, "Invalid graph type", http.StatusBadRequest)
        return
    }
    start := r.URL.Query().Get("start")
    end := r.URL.Query().Get("end")

	var err error
	startdate, err := time.Parse(constants.DateLayout, start)
	if err != nil {
		log.Println("Error while parsing startdate")
		http.Error(w, "Failed to parse parameters", http.StatusBadRequest)
        return 
	}
	enddate, err := time.Parse(constants.DateLayout, end)
	if err != nil {
		log.Println("Error while parsing enddate")
		http.Error(w, "Failed to parse parameters", http.StatusBadRequest)
        return 
	}
	// Iterate through all dates between startdate and enddate and check if the corresponding x and y values are present in Redis
	data := make([]models.DataPoint,0)
	dateNotFoundInRedis := make([]string,0)

	redisSvc := crudredis.GetService()
	if redisSvc != nil {
		data,dateNotFoundInRedis = redisSvc.Get(graphType, startdate, enddate)
	}

	// Fetch data from the database for the dates whose data was not found in Redis
	dataFromDB := make(map[string][]int64)
	if len(dateNotFoundInRedis) > 0 {
        dataFromDB, err = getDataInRangeFromDB(graphType, dateNotFoundInRedis)
		if err != nil {
			log.Println("[getDataInRangeFromDB] err: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _,val := range dataFromDB{
			data = append(data,models.DataPoint{X: val[0], Y:val[1]})
		}
	}

	if redisSvc != nil {
		err = redisSvc.Set(dataFromDB,data,graphType)
		if err != nil {
			log.Println("[Redis] err:",err)
		}
	}

    // Return JSON response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(data)
}

// getDataInRangeFromDB executes a query on the database to fetch data within the specified range.
func getDataInRangeFromDB(graphType string, datanotfoundinredis []string) (map[string][]int64, error) {

	// Define the query to fetch data
    query := `
		SELECT timestamp, x, y 
		FROM graph_data 
		WHERE type = $1 AND timestamp = any($2)
		`
	dbClient := inits.GetDBClient()
	if dbClient == nil {
		log.Println("dbClient not initialized")
		return nil,errors.New("dbClient not initialized")
	}
    rows, err := dbClient.Query(query, graphType, pq.Array(datanotfoundinredis))
    if err != nil {
		log.Println("[getDataInRangeFromDB] Error occurred while executing the query on the database.", err)
        return nil, err
    }
    defer rows.Close()

	data := make(map[string][]int64,0)

	// Iterate over the query results
    for rows.Next() {
        var x, y int64
		var date string
        if err := rows.Scan(&date,&x, &y); err != nil {
            return nil, err
        }
		data[date] = []int64{x,y}
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }
    return data, nil
}