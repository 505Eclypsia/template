package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr                string
	LogLevel                string
	ShutdownTimeout         time.Duration
	DatabaseURL             string
	DatabaseMaxConns        int32
	DatabaseMinConns        int32
	DatabaseMaxConnLifetime time.Duration
	DatabaseConnectTimeout  time.Duration
	DatabaseQueryTimeout    time.Duration

	PositionCheckInterval time.Duration
	PositionStaleAfter    time.Duration
	PositionWorkers       int32
	PositionQueueSize     int32
	PushHTTPURL           string
	PushRequestTimeout    time.Duration
}

type envReader struct {
	errs []error
}

func Load() (*Config, error) {
	r := &envReader{}

	cfg := Config{
		HTTPAddr:                r.string("HTTP_ADDR"),
		LogLevel:                r.string("LOG_LEVEL"),
		ShutdownTimeout:         r.time("SHUTDOWN_TIMEOUT"),
		DatabaseURL:             r.string("DATABASE_URL"),
		DatabaseMaxConns:        r.int32("DATABASE_MAX_CONNS"),
		DatabaseMinConns:        r.int32("DATABASE_MIN_CONNS"),
		DatabaseMaxConnLifetime: r.time("DATABASE_MAX_CONN_LIFETIME"),
		DatabaseConnectTimeout:  r.time("DATABASE_CONNECT_TIMEOUT"),
		DatabaseQueryTimeout:    r.time("DATABASE_QUERY_TIMEOUT"),

		PositionCheckInterval: r.timeDefault("POSITION_CHECK_INTERVAL", 30*time.Second),
		PositionStaleAfter:    r.timeDefault("POSITION_STALE_AFTER", 60*time.Second),
		PositionWorkers:       r.int32Default("POSITION_WORKERS", 4),
		PositionQueueSize:     r.int32Default("POSITION_QUEUE_SIZE", 100),
		PushHTTPURL:           r.string("PUSH_HTTP_URL"),
		PushRequestTimeout:    r.timeDefault("PUSH_REQUEST_TIMEOUT", time.Second),
	}

	if len(r.errs) > 0 {
		return nil, errors.Join(r.errs...)
	}
	return &cfg, nil
}

func getRawString(target string) (string, error) {
	str := os.Getenv(target)
	if str == "" {
		return "", fmt.Errorf("missing required environment variable %s", target)
	}
	return str, nil
}

func (r *envReader) string(target string) string {
	str, err := getRawString(target)
	if err != nil {
		r.errs = append(r.errs, err)
		return ""
	}
	return str
}

func (r *envReader) int32(target string) int32 {
	str, err := getRawString(target)
	if err != nil {
		r.errs = append(r.errs, err)
		return 0
	}

	num, err := strconv.ParseInt(str, 10, 32)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("invalid %s: %w", target, err))
		return 0
	}
	return int32(num)
}

func (r *envReader) int32Default(target string, def int32) int32 {
	if os.Getenv(target) == "" {
		return def
	}
	return r.int32(target)
}

func (r *envReader) timeDefault(target string, def time.Duration) time.Duration {
	if os.Getenv(target) == "" {
		return def
	}
	return r.time(target)
}

func (r *envReader) time(target string) time.Duration {
	str, err := getRawString(target)
	if err != nil {
		r.errs = append(r.errs, err)
		return 0
	}

	dur, err := time.ParseDuration(str)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("invalid %s: %w", target, err))
		return 0
	}
	return dur
}
