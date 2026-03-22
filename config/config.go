package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all environment-based configuration for the application.
// Values are loaded once at startup and passed down to services that need them.
type Config struct {
	// Database connection string built from individual DB env vars
	DatabaseURL string

	// Port the HTTP server will listen on (default: 8080)
	ServerPort string
}

// Load reads the .env file (if present) and populates a Config struct.
// In production, env vars are expected to already be set in the environment.
func Load() *Config {
	// Load .env file — ignore error in production where vars are set externally
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	cfg := &Config{
		DatabaseURL: buildDatabaseURL(),
		ServerPort:  getEnv("SERVER_PORT", "8080"),
	}

	return cfg
}

// buildDatabaseURL constructs a PostgreSQL DSN from individual environment variables.
// This is more explicit and easier to manage in cloud environments than a single URL.
func buildDatabaseURL() string {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "")
	dbname := getEnv("DB_NAME", "ledger_core")
	sslmode := getEnv("DB_SSLMODE", "disable")

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)
}

// getEnv retrieves an environment variable by key.
// Falls back to the provided default value if the variable is not set.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
