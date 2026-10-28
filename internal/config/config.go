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
	DatabaseConnTimeout     time.Duration
	DatabaseQueryTimeout    time.Duration
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
		DatabaseConnTimeout:     r.time("DATABASE_CONNECT_TIMEOUT"),
		DatabaseQueryTimeout:    r.time("DATABASE_QUERY_TIMEOUT"),
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
