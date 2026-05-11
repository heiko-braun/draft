package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/heiko-braun/draft/internal/reviewd"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	debug := false
	for _, arg := range os.Args[1:] {
		if arg == "--debug" || arg == "-debug" {
			debug = true
		}
	}

	cfg, err := reviewd.LoadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if debug {
		cfg.LogLevel = "debug"
	}

	logger := reviewd.NewLogger(cfg.LogLevel)
	logger.Info("starting reviewd", "port", fmt.Sprintf("%d", cfg.Port))

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database open: %w", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connectivity.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}
	logger.Info("database connected")

	// Run migrations.
	if err := reviewd.Migrate(db, logger); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}

	srv := reviewd.NewServer(db, cfg, logger)

	httpServer := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      srv,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.Addr())
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case sig := <-shutdown:
		logger.Info("shutdown signal received", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(ctx)
	case err := <-errCh:
		return err
	}
}
