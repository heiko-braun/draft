package reviewd

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all server configuration, read from environment variables.
type Config struct {
	// DatabaseURL is the Postgres connection string.
	DatabaseURL string

	// Port is the HTTP listen port.
	Port int

	// LogLevel controls log verbosity: "debug", "info", "warn", "error".
	LogLevel string

	// GitHubClientID for OAuth (unused until auth middleware is wired).
	GitHubClientID string

	// GitHubClientSecret for OAuth (unused until auth middleware is wired).
	GitHubClientSecret string
}

// LoadConfig reads configuration from environment variables with sensible defaults.
func LoadConfig() (Config, error) {
	c := Config{
		DatabaseURL:        envOrDefault("DATABASE_URL", "postgres://draft:draft@localhost:5432/draft_reviews?sslmode=disable"),
		Port:               5100,
		LogLevel:           envOrDefault("LOG_LEVEL", "info"),
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
	}

	if portStr := os.Getenv("PORT"); portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return Config{}, fmt.Errorf("invalid PORT %q: %w", portStr, err)
		}
		c.Port = p
	}

	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	return c, nil
}

// Addr returns the listen address as ":port".
func (c Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
