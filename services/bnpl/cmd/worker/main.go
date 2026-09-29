package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	apihttp "gitlab.com/steph-ano/ledgerly/services/bnpl/internal/api/http"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/gateway"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/ledgerclient"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/scheduler"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/storage/postgres"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/webhook"
)

var _ = apihttp.NewServer

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	dbURL := getEnv("DATABASE_URL", "postgres://ledgerly:ledgerly_secret@localhost:5432/ledgerly_dev?sslmode=disable")
	ledgerURL := getEnv("LEDGER_SERVICE_URL", "http://localhost:8080")
	webhookSecret := getEnv("WEBHOOK_SECRET", "ledgerly_whsec_secret_default_key")

	slog.Info("starting ledgerly bnpl dedicated worker",
		"ledger_service_url", ledgerURL,
	)

	db, err := postgres.Open(postgres.DefaultConfig(dbURL))
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	orderRepo := postgres.NewOrderRepository(db)
	attemptRepo := postgres.NewPaymentAttemptRepository(db)
	outboxRepo := postgres.NewOutboxRepository(db)
	gw := gateway.NewSimulator()
	ledgerClient := ledgerclient.NewClient(ledgerURL, 5*time.Second)

	sched := scheduler.NewInstallmentScheduler(
		scheduler.DefaultConfig(),
		orderRepo,
		attemptRepo,
		outboxRepo,
		gw,
		ledgerClient,
	)

	dispatcher := webhook.NewDispatcher(
		webhook.DefaultConfig(webhookSecret),
		outboxRepo,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Scheduler worker loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		slog.Info("dedicated installment scheduler started")
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				count, err := sched.ProcessDueInstallments(ctx, t)
				if err != nil {
					slog.Error("scheduler worker execution error", "error", err)
				} else if count > 0 {
					slog.Info("scheduler worker processed installments", "count", count)
				}
			}
		}
	}()

	// Webhook dispatcher worker loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		slog.Info("dedicated webhook dispatcher started")
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				count, err := dispatcher.ProcessPendingEvents(ctx, t)
				if err != nil {
					slog.Error("webhook dispatcher execution error", "error", err)
				} else if count > 0 {
					slog.Info("webhook dispatcher delivered events", "count", count)
				}
			}
		}
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)

	<-shutdownSignal
	slog.Info("shutdown signal received, terminating worker...")
	cancel()
	wg.Wait()
	slog.Info("bnpl dedicated worker stopped cleanly")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
