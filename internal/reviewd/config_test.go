package reviewd

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear env to test defaults.
	os.Unsetenv("PORT")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("DATABASE_URL")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 5100 {
		t.Errorf("port = %d, want 5100", cfg.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("log_level = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.DatabaseURL == "" {
		t.Error("database_url should have a default")
	}
}

func TestLoadConfig_PortOverride(t *testing.T) {
	os.Setenv("PORT", "9090")
	defer os.Unsetenv("PORT")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("port = %d, want 9090", cfg.Port)
	}
}

func TestLoadConfig_InvalidPort(t *testing.T) {
	os.Setenv("PORT", "not-a-number")
	defer os.Unsetenv("PORT")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestConfig_Addr(t *testing.T) {
	c := Config{Port: 8080}
	if c.Addr() != ":8080" {
		t.Errorf("addr = %q, want %q", c.Addr(), ":8080")
	}
}
