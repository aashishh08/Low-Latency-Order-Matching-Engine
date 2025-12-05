package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           string
	MetricsEnabled bool
	WSEnabled      bool
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		MetricsEnabled: getEnvBool("METRICS_ENABLED", true),
		WSEnabled:      getEnvBool("WS_ENABLED", true),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
