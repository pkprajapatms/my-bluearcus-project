package inits

import (
	"database/sql"
	"fmt"
	"log"
)

var DbClient *sql.DB

func GetDBClient() *sql.DB{
	if DbClient == nil{
		log.Println("hahaha")
	}
	return DbClient
}

// Database connection parameters
const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "12345678"
	dbname   = "mydatabase"
)

// Initialize database connection
func InitDB() {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	var err error
	DbClient, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error connecting to database: %v\n", err)
	}

	// Check if the connection is successful
	err = DbClient.Ping()
	if err != nil {
		log.Fatalf("Error pinging database: %v\n", err)
	}

	log.Println("Connected to database")
}

// Initialize database connection
func CloseDB() {
	// Close the database connection
	if DbClient != nil {
		DbClient.Close()
	}
}