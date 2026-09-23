// Package config provides configuration management for all services
package config

import (
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config holds all configuration for the services
type Config struct {
	// Service
	ServiceName string
	Port        string
	Environment string // development, staging, production

	// Supabase (PostgreSQL)
	SupabaseURL      string
	SupabaseKey      string
	SupabaseDBURL    string // Full connection string for pgx
	SupabaseJWTSecret string

	// NVIDIA NIM (LLM)
	NVIDIANIMURL     string
	NVIDIANIMAPIKey  string
	NVIDIANIMModel   string // e.g., "meta/llama-3.1-70b-instruct"

	// NATS
	NATSURL          string

	// Redis
	RedisURL         string

	// ClickHouse
	ClickHouseURL    string

	// GitHub App
	GitHubAppID          string
	GitHubAppPrivateKey  string
	GitHubWebhookSecret  string
	GitHubClientID       string
	GitHubClientSecret   string

	// Logging
	LogLevel string

	// Rate Limiting
	RateLimitRequests int
	RateLimitWindow   time.Duration
}

// Load loads configuration from environment variables
func Load(serviceName string) *Config {
	return &Config{
		ServiceName:     serviceName,
		Port:            getEnv("PORT", "8080"),
		Environment:     getEnv("ENVIRONMENT", "development"),

		// Supabase
		SupabaseURL:       getEnv("SUPABASE_URL", ""),
		SupabaseKey:       getEnv("SUPABASE_KEY", ""),
		SupabaseDBURL:     getEnv("SUPABASE_DB_URL", ""),
		SupabaseJWTSecret: getEnv("SUPABASE_JWT_SECRET", ""),

		// NVIDIA NIM
		NVIDIANIMURL:    getEnv("NVIDIA_NIM_URL", "https://integrate.api.nvidia.com/v1"),
		NVIDIANIMAPIKey: getEnv("NVIDIA_NIM_API_KEY", ""),
		NVIDIANIMModel:  getEnv("NVIDIA_NIM_MODEL", "meta/llama-3.1-70b-instruct"),

		// NATS
		NATSURL: getEnv("NATS_URL", "nats://localhost:4222"),

		// Redis
		RedisURL: getEnv("REDIS_URL", "redis://localhost:6379"),

		// ClickHouse
		ClickHouseURL: getEnv("CLICKHOUSE_URL", "http://localhost:8123"),

		// GitHub App
		GitHubAppID:         getEnv("GITHUB_APP_ID", ""),
		GitHubAppPrivateKey: getEnv("GITHUB_APP_PRIVATE_KEY", ""),
		GitHubWebhookSecret: getEnv("GITHUB_WEBHOOK_SECRET", ""),
		GitHubClientID:      getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:  getEnv("GITHUB_CLIENT_SECRET", ""),

		// Logging
		LogLevel: getEnv("LOG_LEVEL", "info"),

		// Rate Limiting
		RateLimitRequests: getEnvAsInt("RATE_LIMIT_REQUESTS", 1000),
		RateLimitWindow:   getEnvAsDuration("RATE_LIMIT_WINDOW", time.Minute),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// SetupLogger creates a configured zap logger
func SetupLogger(level string) (*zap.Logger, error) {
	var zapLevel zap.AtomicLevel
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	config := zap.NewProductionConfig()
	config.Level = zapLevel
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return config.Build()
}