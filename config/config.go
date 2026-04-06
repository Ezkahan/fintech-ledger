package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPPort    string
	PostgresDSN string
	RedisAddr   string
	RedisPass   string
	RedisDB     int
}

func Load() *Config {
	return &Config{
		HTTPPort:    getEnv("HTTP_PORT", "8080"),
		PostgresDSN: getEnv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/ledger?sslmode=disable"),
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPass:   getEnv("REDIS_PASS", ""),
		RedisDB:     getEnvInt("REDIS_DB", 0),
	}
}

func getEnv(key, def string) string {
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
