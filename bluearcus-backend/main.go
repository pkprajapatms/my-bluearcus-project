package main

import (
	"bluearcus-backend/inits"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"

	clearredis "bluearcus-backend/controllers/clear-redis"
	cruddb "bluearcus-backend/controllers/crud-db"
	websocket "bluearcus-backend/controllers/web-socket"
	crudredis "bluearcus-backend/services/crud-redis"
	redisSvc "bluearcus-backend/services/crud-redis/crudredis"
)

func init() {
	inits.InitRedis()
	inits.InitDB()
	inits.InitWebSocket()
}

func main() {

	redisClient := inits.GetRedisClient()
	redisSvc := redisSvc.Service{
		Client: redisClient,
	}
	
	crudredis.Init(redisSvc)

	router := routerHandler()

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
	close(inits.CloseSocketchannel)
	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the server gracefully
	if err := server.Shutdown(ctx); err != nil {
		log.Println("Error shutting down server:", err)
	}

	inits.CloseDB()
}

func routerHandler() *mux.Router{
	// Initialize the router
	router := mux.NewRouter()
	router.HandleFunc("/data/range", cruddb.GetDataInRange).Methods("GET")
	router.HandleFunc("/data/add-data", cruddb.AddData).Methods("POST")
	router.HandleFunc("/ws", websocket.WebsocketHandler)
	router.HandleFunc("/data/{graph}", cruddb.GetData).Methods("GET")
	router.HandleFunc("/cache/clear-cache", clearredis.ClearCache).Methods("POST")
	return router
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





