package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	Database DatabaseConfig
	Queue    QueueConfig
	API      APIConfig
	Worker   WorkerConfig
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// QueueConfig holds queue configuration (Redis)
type QueueConfig struct {
	RedisURL  string
	QueueName string
}

// APIConfig holds API server configuration
type APIConfig struct {
	Port int
}

// WorkerConfig holds worker configuration
type WorkerConfig struct {
	Concurrency   int
	MaxRetryCount int
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	dbPort, err := strconv.Atoi(getEnv("DB_PORT", "5432"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_PORT: %w", err)
	}

	apiPort, err := strconv.Atoi(getEnv("API_PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid API_PORT: %w", err)
	}

	workerConcurrency, err := strconv.Atoi(getEnv("WORKER_CONCURRENCY", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_CONCURRENCY: %w", err)
	}

	maxRetryCount, err := strconv.Atoi(getEnv("MAX_RETRY_COUNT", "3"))
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_RETRY_COUNT: %w", err)
	}

	return &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "campaign_manager"),
			Password: getEnv("DB_PASSWORD", "campaign_manager"),
			DBName:   getEnv("DB_NAME", "campaign_manager"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Queue: QueueConfig{
			RedisURL:  getEnv("REDIS_URL", "redis://localhost:6379/0"),
			QueueName: getEnv("QUEUE_NAME", "campaign_sends"),
		},
		API: APIConfig{
			Port: apiPort,
		},
		Worker: WorkerConfig{
			Concurrency:   workerConcurrency,
			MaxRetryCount: maxRetryCount,
		},
	}, nil
}

// DSN returns the database connection string
func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
