package main

import (
	"fmt"
	"log"

	"github.com/GordenArcher/ledger-core/config"
	"github.com/GordenArcher/ledger-core/internal/account"
	"github.com/GordenArcher/ledger-core/internal/idempotency"
	"github.com/GordenArcher/ledger-core/internal/ledger"
	"github.com/GordenArcher/ledger-core/internal/transfer"
	"github.com/GordenArcher/ledger-core/pkg/database"
	"github.com/GordenArcher/ledger-core/pkg/middleware"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	db := database.Connect(cfg.DatabaseURL)

	// Auto-migrate all models including the new idempotency records table
	if err := db.AutoMigrate(
		&account.Account{},
		&transfer.Transfer{},
		&ledger.Entry{},
		&idempotency.Record{},
	); err != nil {
		log.Fatalf("Auto-migration failed: %v", err)
	}
	log.Println("Database migration complete")

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "ledger-core"})
	})

	// Build dependency graph
	ledgerRepo := ledger.NewRepository(db)
	ledgerService := ledger.NewService(ledgerRepo)

	accountRepo := account.NewRepository(db)
	accountService := account.NewService(accountRepo, ledgerService)

	transferRepo := transfer.NewRepository(db)
	transferService := transfer.NewService(db, transferRepo, accountRepo, ledgerService)

	idempotencyRepo := idempotency.NewRepository(db)

	// API v1 group
	v1 := router.Group("/api/v1")

	// Apply idempotency middleware only to POST routes.
	// GET routes are read-only and don't need deduplication.
	v1.Use(func(c *gin.Context) {
		if c.Request.Method == "POST" {
			middleware.Idempotency(idempotencyRepo)(c)
		} else {
			c.Next()
		}
	})

	account.RegisterRoutes(v1, accountService)
	transfer.RegisterRoutes(v1, transferService)
	ledger.RegisterRoutes(v1, ledgerService)

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("ledger-core starting on %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
