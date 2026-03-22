package database

import (
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global GORM database instance.
// It is initialized once at startup and reused across all repositories.
var DB *gorm.DB

// Connect establishes a connection to PostgreSQL using the provided DSN
// and assigns the result to the global DB variable.
// It panics on failure since the app cannot run without a DB connection.
func Connect(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		// Use Info level logging in development to see all SQL queries.
		// Switch to logger.Silent in production to reduce noise.
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Database connection established")

	DB = db
	return db
}
