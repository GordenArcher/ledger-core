package main

import (
	"fmt"
	"log"

	"github.com/GordenArcher/ledger-core/config"
	"github.com/GordenArcher/ledger-core/internal/account"
	"github.com/GordenArcher/ledger-core/pkg/database"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load all configuration from environment / .env file
	cfg := config.Load()

	// Connect to PostgreSQL — will panic if connection fails
	db := database.Connect(cfg.DatabaseURL)

	// Run GORM auto-migrations — creates or updates tables to match models.
	// Safe to run on every startup; GORM only adds missing columns/indexes,
	// it never drops existing ones.
	if err := db.AutoMigrate(
		&account.Account{},
		// TODO: add &transaction.Transaction{}, &transfer.Transfer{} as we build them
	); err != nil {
		log.Fatalf("Auto-migration failed: %v", err)
	}
	log.Println("Database migration complete")

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

	// API v1 route group — all domain routes live under /api/v1
	v1 := router.Group("/api/v1")

	// Wire up account routes with their dependencies
	// Handler → Service → Repository → DB
	accountRepo := account.NewRepository(db)
	accountService := account.NewService(accountRepo)
	account.RegisterRoutes(v1, accountService)

	// TODO: wire transfer and reconciliation routes as we build them

	// Start the HTTP server on the configured port
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("ledger-core starting on %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
