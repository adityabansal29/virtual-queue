package config

import (
	"os"
	"strconv"
	"time"
)

// ---------------------------------------------------------------------------
// TTL constants — single source of truth for all Redis key and cookie lifetimes.
// ---------------------------------------------------------------------------

const (
	// Redis key TTLs
	TicketKeyTTL     = 2 * time.Hour    // ticket:{ticketID} hash — expires if never admitted
	AdmissionUsedTTL = 30 * time.Minute // token:{jti} one-time SETNX (TOKEN-04)
	SchedulerLockTTL = 10 * time.Second // scheduler:lock:{eventID} leader lock

	// JWT lifetimes
	AdmissionJWTTTL = 30 * time.Minute
	SessionJWTTTL   = 30 * time.Minute

	// Cookie max-age in seconds (as required by http.SetCookie / gin.SetCookie).
	// queue.js uses QAdmissionCookieMaxAge value (1800) as a literal string.
	QTicketCookieMaxAge    = 3600 // q_ticket:    1 hour
	QAdmissionCookieMaxAge = 1800 // q_admission: 30 minutes
	QSessionCookieMaxAge   = 1800 // q_session:   30 minutes
)

// ---------------------------------------------------------------------------
// Per-service config types
// ---------------------------------------------------------------------------

// QueueServerConfig holds config for the HTTP queue API server.
// No secrets: the queue server does not sign or verify any tokens.
type QueueServerConfig struct {
	RedisAddr           string
	Port                string
	SSEThreshold        int
	DefaultAdmitRate    int64
	QueuePageURL        string
	QueuePageBucketName string
}

// SchedulerConfig holds config for the admission scheduler process.
type SchedulerConfig struct {
	RedisAddr        string
	AdmissionSecret  string
	DefaultAdmitRate int64
	TickSecs         int
}

// StubOriginConfig holds config for the stub checkout origin.
type StubOriginConfig struct {
	RedisAddr       string
	AdmissionSecret string
	SessionSecret   string
	QueueJoinURL    string
	Secure          bool
}

// ---------------------------------------------------------------------------
// Load functions
// ---------------------------------------------------------------------------

func LoadQueueServer() QueueServerConfig {
	return QueueServerConfig{
		RedisAddr:           getEnvOrDefault("REDIS_ADDR", "redis-queue:6379"),
		Port:                getEnvOrDefault("PORT", "8080"),
		SSEThreshold:        getEnvInt("SSE_THRESHOLD", 200),
		DefaultAdmitRate:    getEnvInt64("DEFAULT_ADMIT_RATE", 60),
		QueuePageURL:        getEnvOrDefault("QUEUE_PAGE_URL", "http://localhost:8082/queue/"),
		QueuePageBucketName: getEnvOrDefault("QUEUE_PAGE_BUCKET_NAME", ""),
	}
}

func LoadScheduler() SchedulerConfig {
	secret := os.Getenv("ADMISSION_SECRET")
	if secret == "" {
		panic("ADMISSION_SECRET must be set and non-empty")
	}
	return SchedulerConfig{
		RedisAddr:        getEnvOrDefault("REDIS_ADDR", "redis-queue:6379"),
		AdmissionSecret:  secret,
		DefaultAdmitRate: getEnvInt64("DEFAULT_ADMIT_RATE", 60),
		TickSecs:         getEnvInt("SCHEDULER_TICK_SECS", 1),
	}
}

func LoadStubOrigin() StubOriginConfig {
	admissionSecret := os.Getenv("ADMISSION_SECRET")
	if admissionSecret == "" {
		panic("ADMISSION_SECRET must be set and non-empty")
	}
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		panic("SESSION_SECRET must be set and non-empty")
	}
	if admissionSecret == sessionSecret {
		panic("ADMISSION_SECRET and SESSION_SECRET must be different values")
	}
	return StubOriginConfig{
		RedisAddr:       getEnvOrDefault("REDIS_ADDR", "redis-origin:6379"),
		AdmissionSecret: admissionSecret,
		SessionSecret:   sessionSecret,
		QueueJoinURL:    getEnvOrDefault("QUEUE_JOIN_URL", "http://localhost:8080/queue/join"),
		Secure:          getEnvBool("SECURE", false),
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

func getEnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
