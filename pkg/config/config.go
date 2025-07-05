package config

import (
	"os"
	"strconv"

	"hpa-monitor/pkg/logger"
)

// Config holds application configuration
type Config struct {
	Port              string
	Tolerance         float64
	WebSocketInterval int    // seconds
	LogLevel          string // debug, info, warn, error, fatal, panic
}

// NewConfig creates a new configuration from environment variables
func NewConfig() *Config {
	config := &Config{
		Port:              getEnv("PORT", "8080"),
		Tolerance:         getFloatEnv("TOLERANCE", 0.1),
		WebSocketInterval: getIntEnv("WEBSOCKET_INTERVAL", 5),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
	}

	// Initialize logger with configured log level
	logger.InitLogger(config.LogLevel)

	// Log configuration
	log := logger.GetLogger()
	log.WithFields(logger.Fields{
		"port":              config.Port,
		"tolerance":         config.Tolerance,
		"websocket_interval": config.WebSocketInterval,
		"log_level":         config.LogLevel,
	}).Info("Configuration loaded")

	return config
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getFloatEnv gets a float environment variable with a default value
func getFloatEnv(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

// getIntEnv gets an int environment variable with a default value
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}