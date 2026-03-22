package main

import (
	"fmt"
	"log"

	"github.com/GordenArcher/ledger-core/config"
	"github.com/GordenArcher/ledger-core/pkg/database"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load all configuration from environment / .env file
	cfg := config.Load()

	// Connect to PostgreSQL — will panic if connection fails
	database.Connect(cfg.DatabaseURL)

	// Initialize the Gin router with default middleware:
	// - Logger: logs each request with latency and status
	// - Recovery: recovers from panics and returns a 500
	router := gin.Default()

	// Health check endpoint — useful for verifying the server is up
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "ledger-core",
		})
	})

	// Start the HTTP server on the configured port
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("ledger-core starting on %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
