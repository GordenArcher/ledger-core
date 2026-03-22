package main

import (
	"fmt"
	"log"

	"github.com/GordenArcher/ledger-core/config"
	"github.com/GordenArcher/ledger-core/internal/account"
	"github.com/GordenArcher/ledger-core/internal/ledger"
	"github.com/GordenArcher/ledger-core/internal/transfer"
	"github.com/GordenArcher/ledger-core/pkg/database"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load all configuration from environment / .env file
	cfg := config.Load()

	// Connect to PostgreSQL
	db := database.Connect(cfg.DatabaseURL)

	// Auto-migrate all models
	if err := db.AutoMigrate(
		&account.Account{},
		&transfer.Transfer{},
		&ledger.Entry{},
	); err != nil {
		log.Fatalf("Auto-migration failed: %v", err)
	}
	log.Println("Database migration complete")

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "ledger-core"})
	})

	v1 := router.Group("/api/v1")

	// Build dependency graph bottom-up:
	// ledger has no dependencies on other domains
	ledgerRepo := ledger.NewRepository(db)
	ledgerService := ledger.NewService(ledgerRepo)

	// account depends on ledger for writing entries
	accountRepo := account.NewRepository(db)
	accountService := account.NewService(accountRepo, ledgerService)
	account.RegisterRoutes(v1, accountService)

	// transfer depends on both account repo and ledger
	transferRepo := transfer.NewRepository(db)
	transferService := transfer.NewService(db, transferRepo, accountRepo, ledgerService)
	transfer.RegisterRoutes(v1, transferService)

	// ledger endpoints (account statement)
	ledger.RegisterRoutes(v1, ledgerService)

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("ledger-core starting on %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
