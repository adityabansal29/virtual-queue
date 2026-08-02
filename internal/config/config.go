package config

import (
	"os"
	"strconv"
)

// Config holds all runtime configuration for the queue server.
type Config struct {
	RedisAddr      string
	Port           string
	AdmissionSecret string
	SessionSecret   string
	SSEThreshold    int
	DefaultEventID  string
	DefaultAdmitRate int64
}

// Load reads configuration from environment variables.
// Panics at startup if ADMISSION_SECRET or SESSION_SECRET are missing or identical.
// ponytail: os.Getenv is sufficient; no dotenv library needed for Phase 1.
func Load() Config {
	cfg := Config{
		RedisAddr:        getEnvOrDefault("REDIS_ADDR", "redis-queue:6379"),
		Port:             getEnvOrDefault("PORT", "8080"),
		AdmissionSecret:  os.Getenv("ADMISSION_SECRET"),
		SessionSecret:    os.Getenv("SESSION_SECRET"),
		SSEThreshold:     getEnvInt("SSE_THRESHOLD", 200),
		DefaultEventID:   getEnvOrDefault("DEFAULT_EVENT_ID", "evt-001"),
		DefaultAdmitRate: getEnvInt64("DEFAULT_ADMIT_RATE", 60),
	}

	// T-01-03 + TOKEN-02: secrets must be non-empty and distinct.
	if cfg.AdmissionSecret == "" {
		panic("ADMISSION_SECRET must be set and non-empty")
	}
	if cfg.SessionSecret == "" {
		panic("SESSION_SECRET must be set and non-empty")
	}
	if cfg.AdmissionSecret == cfg.SessionSecret {
		panic("ADMISSION_SECRET and SESSION_SECRET must be different values")
	}

	return cfg
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}
