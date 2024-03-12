package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"database/sql"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/lib/pq"
)

var (
	redisClient        *redis.Client
	db                 *sql.DB
	Socketchannel      = make(chan []byte)
	CloseSocketchannel = make(chan bool)
)

// Database connection parameters
const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "12345678"
	dbname   = "mydatabase"
)

func initRedis() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
}

// Initialize database connection
func initDB() {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error connecting to database: %v\n", err)
	}

	// Check if the connection is successful
	err = db.Ping()
	if err != nil {
		log.Fatalf("Error pinging database: %v\n", err)
	}

	log.Println("Connected to database")
}

func init() {
	initRedis()
	initDB()
}

func main() {
	// Initialize the router
	router := mux.NewRouter()
	router.HandleFunc("/data/range", getDataInRange).Methods("GET")
	router.HandleFunc("/data/add-data", addData).Methods("POST")
	router.HandleFunc("/ws", websocketHandler)
	router.HandleFunc("/data/{graph}", getData).Methods("GET")
	router.HandleFunc("/cache/clear-cache", clearCache).Methods("POST")

	server := &http.Server{
		Addr:    ":8080",
		Handler: addCorsHeaders(router),
	}

	// Start the server in a separate goroutine
	go func() {
		log.Println("Server is listening on port 8080...")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println("Error starting server:", err)
		}
	}()

	// Setup signal handling to gracefully shutdown the server
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Wait for SIGINT (Ctrl+C) signal
	<-interrupt
	CloseSocketchannel <- true
	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the server gracefully
	if err := server.Shutdown(ctx); err != nil {
		log.Println("Error shutting down server:", err)
	}

	// Close the database connection
	if db != nil {
		db.Close()
	}
}

// Middleware function to add CORS headers
func addCorsHeaders(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from any origin
		w.Header().Set("Access-Control-Allow-Origin", "*")
		// Allow GET, POST, PUT, DELETE methods
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
		// Allow the Content-Type header
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		// Allow credentials
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// If it's a preflight request, respond with 200 OK
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		handler.ServeHTTP(w, r)
	})
}

type DataPoint struct {
	X int64 `json:"x"`
	Y int64 `json:"y"`
}

// getDataInRange handles range queries on the dataset.
// It retrieves data points within the specified date range for a given graph type.
// The date range is specified using the "start" and "end" query parameters.
// The response contains a JSON array of DataPoint objects.
func getDataInRange(w http.ResponseWriter, r *http.Request){

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

    data := make([]DataPoint,0)
	var err error
	startdate, err := time.Parse(dateLayout, start)
	if err != nil {
		log.Println("Error while parsing startdate")
		http.Error(w, "Failed to parse parameters", http.StatusBadRequest)
        return 
	}
	enddate, err := time.Parse(dateLayout, end)
	if err != nil {
		log.Println("Error while parsing enddate")
		http.Error(w, "Failed to parse parameters", http.StatusBadRequest)
        return 
	}
	var tempdata DataPoint

	// WaitGroup to wait for goroutines to finish
	var wg sync.WaitGroup
	var dataMutex sync.Mutex
	// Iterate through all dates between startdate and enddate and check if the corresponding x and y values are present in Redis
	dateNotFoundInRedis := make([]string,0)
	for current := startdate; current.Before(enddate) || current.Equal(enddate); current = current.AddDate(0, 0, 1) {
		cacheKey := graphType + current.Format(dateLayout)
		wg.Add(1)

		go func(cacheKey string) {
			defer wg.Done()
			val, err := redisClient.Get(context.Background(), cacheKey).Result()
			if err == nil {
				// Value found in Redis, parse and add it to data slice
				err = json.Unmarshal([]byte(val),&tempdata)
				if err == nil {
					dataMutex.Lock()
					data = append(data,tempdata)
					dataMutex.Unlock()
				}
			} else if err == redis.Nil {
				// Value not found in Redis, add date to list of dates not found
				dataMutex.Lock()
				dateNotFoundInRedis = append(dateNotFoundInRedis,current.Format(dateLayout))
				dataMutex.Unlock()
			} else {
				// Error occurred while fetching from Redis
				log.Println("Error fetching data from Redis:", err)
			}
		}(cacheKey)
	}
	wg.Wait()

	// Fetch data from the database for the dates whose data was not found in Redis
	dataFromDB := make(map[string][]int64)
	if len(dateNotFoundInRedis) > 0 {
        dataFromDB, err = getDataInRangeFromDB(db, graphType, dateNotFoundInRedis)
		if err != nil {
			log.Println("[getDataInRangeFromDB] err: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Adding data fetched from the database to Redis
	for key,val := range dataFromDB{
		t,_ := time.Parse(dbDatelayout, key)
		cacheKey := graphType + t.Format(dateLayout)
		wg.Add(1)

		go func(cacheKey string) {
			defer wg.Done()
			jsondata,_ := json.Marshal(DataPoint{X:val[0],Y:val[1]})
			data = append(data,DataPoint{X:val[0],Y:val[1]})
			redisClient.Set(context.Background(), cacheKey, string(jsondata), 60*time.Minute) // Cache data for 10 minutes
		}(cacheKey)
	}
	wg.Wait()

    // Return JSON response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(data)
}

// getDataInRangeFromDB executes a query on the database to fetch data within the specified range.
func getDataInRangeFromDB(db *sql.DB, graphType string, datanotfoundinredis []string) (map[string][]int64, error) {

	// Define the query to fetch data
    query := `
		SELECT timestamp, x, y 
		FROM graph_data 
		WHERE type = $1 AND timestamp = any($2)
		`
    rows, err := db.Query(query, graphType, pq.Array(datanotfoundinredis))
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

//getData: retrieves data based on the graph type for all available time periods.
func getData(w http.ResponseWriter, r *http.Request) {
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	cancel()

	graphType := mux.Vars(r)["graph"]

	// Try to fetch data from Redis cache
	cacheKey := "data:" + graphType
	rows, err := db.Query("SELECT x, y FROM graph_data WHERE type = $1", graphType)

	if err != nil {
		log.Printf("Error querying database: %v\n", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Iterate through the rows and construct a slice of DataPoint
	var data []DataPoint
	for rows.Next() {
		var dp DataPoint
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
	redisClient.Set(ctx, cacheKey, string(jsonData), 10*time.Minute) // Cache data for 10 minutes

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// clearCache handles the request to clear the cache.
func clearCache(w http.ResponseWriter, r *http.Request) {

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Clear the cache
    err := redisClient.FlushAll(ctx).Err()
    if err != nil {
        http.Error(w, "Error clearing cache", http.StatusInternalServerError)
        return
    }

    // Respond with a success message
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"message": "Cache cleared successfully"})
}

type InsertData struct {
	X         int64  `json:"x"`
	Y         int64  `json:"y"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
}

// curl --location --request POST 'http://localhost:8080/cache/add-data?x=6&y=4&timestamp=2024-03-11T12%3A00%3A00Z&type=line' \
// --header 'Content-Type: application/json' \
// --data ''
func addData(w http.ResponseWriter, r *http.Request) {
	// Decode JSON request body into the Data struct
	var newData InsertData
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
	newData = InsertData{x, y, timestamp, graphType}

	// Check if the data already exists in the database
	if exists, err := dataExistsInDB(graphType, timestamp); err != nil {
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
	jsondata, _ := json.Marshal(InsertData{X: x, Y: y, Timestamp: timestamp, Type: graphType})
	Socketchannel <- jsondata
	w.WriteHeader(http.StatusCreated)
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
	err := db.QueryRow(query, graphType, timestamp).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

//insertDataIntoDB : insert data into DB
func insertDataIntoDB(data InsertData) error {
    query := `
        INSERT INTO graph_data (x, y, timestamp, type)
        VALUES ($1, $2, $3,$4)
    `
    // Execute the SQL query with the provided data
    _, err := db.Exec(query, data.X, data.Y, data.Timestamp, data.Type)
    if err != nil {
        return err
    }

    return nil
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all connections by returning true
		return true
	},
}

// ImplementingWebSocketForLiveUpdate initializes WebSocket functionality for live updates.
func websocketHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade connection to WebSocket", http.StatusBadRequest)
		return
	}
	defer conn.Close() // Close the WebSocket connection when the function returns

	// Handle WebSocket messages
	go func() {
		defer conn.Close() // Close the WebSocket connection when the goroutine exits
		for {
			select {
			case broadcast := <-Socketchannel:
				// Write the message to the WebSocket connection
				err := conn.WriteMessage(websocket.TextMessage, broadcast)
				if err != nil {
					log.Println("Error writing to WebSocket:", err)
					return // Exit the goroutine if an error occurs
				}
			case <-CloseSocketchannel:
				close(Socketchannel)
				close(CloseSocketchannel)
				log.Println("WebSocket shut down")
				return // Exit the goroutine if CloseSocketchannel is closed
			}
		}
	}()
}








