package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	RedisAddr      string
	DataDir        string
	Processor      string
	ProcessTimeout time.Duration
	MaxAttempts    int
	APIPort        string
	WorkerPort     string
}

func Load() Config {
	return Config{
		RedisAddr:      env("CUDAOPS_REDIS_ADDR", "localhost:6379"),
		DataDir:        env("CUDAOPS_DATA_DIR", "data"),
		Processor:      env("CUDAOPS_PROCESSOR", "cudaops-process"),
		ProcessTimeout: durationEnv("CUDAOPS_PROCESS_TIMEOUT", 60*time.Second),
		MaxAttempts:    intEnv("CUDAOPS_MAX_ATTEMPTS", 2),
		APIPort:        env("CUDAOPS_API_PORT", "8080"),
		WorkerPort:     env("CUDAOPS_WORKER_PORT", "8081"),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}

func intEnv(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
