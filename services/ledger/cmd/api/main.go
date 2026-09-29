package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	apihttp "gitlab.com/steph-ano/ledgerly/services/ledger/internal/api/http"
	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/service"
	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/storage/postgres"
	"gitlab.com/steph-ano/ledgerly/services/ledger/migrations"
)

func main() {
	// Configure structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "postgres://ledgerly:ledgerly_secret@localhost:5432/ledgerly_dev?sslmode=disable")
	overdraftLimitStr := getEnv("PLATFORM_OVERDRAFT_LIMIT_CENTS", "0")
	overdraftLimit, _ := strconv.ParseInt(overdraftLimitStr, 10, 64)

	slog.Info("starting ledgerly ledger service",
		"port", port,
		"overdraft_limit_cents", overdraftLimit,
	)

	// Connect to database
	db, err := postgres.Open(postgres.DefaultConfig(dbURL))
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Apply migrations on startup
	ctxMigrate, cancelMigrate := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelMigrate()

	if err := postgres.ExecuteMigrationScript(ctxMigrate, db, migrations.UpSQL); err != nil {
		slog.Error("failed to execute database migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("database migrations applied successfully")

	// Initialize repositories and service
	accountRepo := postgres.NewAccountRepository(db)
	txRepo := postgres.NewTransactionRepository(db)
	ledgerSvc := service.NewLedgerService(accountRepo, txRepo, overdraftLimit)

	server := apihttp.NewServer(ledgerSvc, db)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      server.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown channel
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("http server listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-shutdownSignal
	slog.Info("shutdown signal received, gracefully terminating...")

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := httpServer.Shutdown(ctxShutdown); err != nil {
		slog.Error("http server graceful shutdown failed", "error", err)
	}

	slog.Info("ledger service stopped cleanly")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
