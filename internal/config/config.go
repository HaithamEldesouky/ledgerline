// Package config loads and validates application configuration from the
// environment, following the 12-factor methodology. Invalid configuration
// fails fast at startup rather than surfacing as a runtime bug.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-validated application configuration.
type Config struct {
	Host            string
	Port            int
	Env             string
	LogLevel        string
	StorageDriver   string
	DatabaseURL     string
	RateLimitRPS    float64
	RateLimitBurst  int
	ShutdownTimeout time.Duration
}

const (
	driverMemory   = "memory"
	driverPostgres = "postgres"
)

var (
	validEnvs      = map[string]bool{"development": true, "test": true, "production": true}
	validLogLevels = map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	validDrivers   = map[string]bool{driverMemory: true, driverPostgres: true}
)

// Load reads configuration from the process environment and validates it.
func Load() (Config, error) {
	port, err := getEnvInt("PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	rps, err := getEnvFloat("RATE_LIMIT_RPS", 100)
	if err != nil {
		return Config{}, err
	}
	burst, err := getEnvInt("RATE_LIMIT_BURST", 200)
	if err != nil {
		return Config{}, err
	}
	shutdown, err := getEnvDuration("SHUTDOWN_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Host:            getEnv("HOST", "0.0.0.0"),
		Port:            port,
		Env:             strings.ToLower(getEnv("APP_ENV", "development")),
		LogLevel:        strings.ToLower(getEnv("LOG_LEVEL", "info")),
		StorageDriver:   strings.ToLower(getEnv("STORAGE_DRIVER", driverMemory)),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		RateLimitRPS:    rps,
		RateLimitBurst:  burst,
		ShutdownTimeout: shutdown,
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("PORT must be between 1 and 65535, got %d", c.Port)
	}
	if !validEnvs[c.Env] {
		return fmt.Errorf("APP_ENV must be one of development|test|production, got %q", c.Env)
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("LOG_LEVEL must be one of debug|info|warn|error, got %q", c.LogLevel)
	}
	if !validDrivers[c.StorageDriver] {
		return fmt.Errorf("STORAGE_DRIVER must be one of memory|postgres, got %q", c.StorageDriver)
	}
	if c.StorageDriver == driverPostgres && c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required when STORAGE_DRIVER is %q", driverPostgres)
	}
	if c.RateLimitRPS <= 0 {
		return fmt.Errorf("RATE_LIMIT_RPS must be positive, got %v", c.RateLimitRPS)
	}
	if c.RateLimitBurst < 1 {
		return fmt.Errorf("RATE_LIMIT_BURST must be at least 1, got %d", c.RateLimitBurst)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func getEnvFloat(key string, fallback float64) (float64, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number: %w", key, err)
	}
	return parsed, nil
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration (e.g. 15s): %w", key, err)
	}
	return parsed, nil
}
