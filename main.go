package main

import (
	"awesomeProject/internal"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
)

const (
	// all of these variables should be set as environment variables
	dbHost     = "127.0.0.1"
	dbPort     = "5432"
	dbUser     = "admin"
	dbPassword = "admin"
	dbName     = "database"
	serverAddr = ":8080"
)

func main() {
	// Initialize a logger
	logger := logrus.New()

	// Create a new database connection
	store, err := internal.NewDatabase(dbHost, dbPort, dbUser, dbPassword, dbName)
	if err != nil {
		logger.Fatalf("Failed to create a database connection: %v", err)
	}

	// Get the current working directory
	dir, err := os.Getwd()
	if err != nil {
		logger.Fatalf("Error getting the current working directory: %v", err)
		return
	}

	// Run database migrations
	err = store.RunMigrations(dir + "/migrations")
	if err != nil {
		logger.Fatalf("Failed to run database migrations: %v", err)
	}

	// Create a new DailyDataUpdater instance
	ddu := internal.NewDailyDataUpdater("https://www.ssga.com", store, logger)

	go ddu.Run()

	// Create a new server
	server := internal.NewServer(logger, store)

	// Create HTTP handlers
	handlers := internal.NewHandler(server, []byte("something"))

	// Create a router and set up routes
	r := internal.MakeHTTPHandler(handlers)

	// Start the HTTP server
	err = http.ListenAndServe(serverAddr, r)
	if err != nil {
		logger.Fatalf("Failed to start the HTTP server: %v", err)
	}
}
