package cruddb

import (
	"bluearcus-backend/inits"
	"bluearcus-backend/models"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
)

// curl --location --request POST 'http://localhost:8080/cache/add-data?x=6&y=4&timestamp=2024-03-11T12%3A00%3A00Z&type=line' \
// --header 'Content-Type: application/json' \
// --data ''
func AddData(w http.ResponseWriter, r *http.Request) {
	// Decode JSON request body into the Data struct
	var newData models.InsertData
	var err error
	var x, y int64
	var graphType, timestamp string
	x, err = strconv.ParseInt(r.URL.Query().Get("x"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	y, err = strconv.ParseInt(r.URL.Query().Get("y"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	graphType = r.URL.Query().Get("type")
	timestamp = r.URL.Query().Get("timestamp")
	newData = models.InsertData{X:x, Y:y, Timestamp: timestamp, Type:graphType}

	// Check if the data already exists in the database
	if exists, err := dataExistsInDB(graphType, timestamp); err != nil {
		log.Println("[dataExistsInDB] err: ",err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if exists {
		http.Error(w, "Data for the given timestamp already exists", http.StatusBadRequest)
		return
	}

	// Insert the received data into the database
	err = insertDataIntoDB(newData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with success message
	jsondata, err := json.Marshal(models.InsertData{X: x, Y: y, Timestamp: timestamp, Type: graphType})
	if err != nil {
		log.Println("AddData response marshal failed")
	}
	inits.Socketchannel <- jsondata
	w.WriteHeader(http.StatusCreated)
}

//insertDataIntoDB : insert data into DB
func insertDataIntoDB(data models.InsertData) error {
    query := `
        INSERT INTO graph_data (x, y, timestamp, type)
        VALUES ($1, $2, $3,$4)
    `
    // Execute the SQL query with the provided data
	dbClient := inits.GetDBClient()
	if dbClient == nil{
		log.Println("Db not initialized")
		return errors.New("Db not initialized")
	}
    _, err := dbClient.Exec(query, data.X, data.Y, data.Timestamp, data.Type)
    if err != nil {
		log.Println("Insert query failed")
        return err
    }

    return nil
}


// dataExistsInDB checks if data with the given graph type and timestamp already exists in the database.
func dataExistsInDB(graphType, timestamp string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM graph_data
			WHERE type = $1 AND timestamp = $2
		)
	`
	var exists bool
	dbClient := inits.GetDBClient()
	if dbClient == nil {
		log.Println("db client is nil")
		return false, errors.New("DB not initialized")
	}
	err := dbClient.QueryRow(query, graphType, timestamp).Scan(&exists)
	if err != nil {
		log.Println("Exist query failed")
		return false, err
	}
	return exists, nil
}
