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

	"github.com/google/uuid"

	apihttp "gitlab.com/steph-ano/ledgerly/services/bnpl/internal/api/http"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/gateway"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/ledgerclient"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/scheduler"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/service"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/storage/postgres"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/webhook"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	port := getEnv("PORT", "8081")
	dbURL := getEnv("DATABASE_URL", "postgres://ledgerly:ledgerly_secret@localhost:5432/ledgerly_dev?sslmode=disable")
	ledgerURL := getEnv("LEDGER_SERVICE_URL", "http://localhost:8080")

	platformAccountIDStr := getEnv("PLATFORM_ACCOUNT_ID", "11111111-1111-1111-1111-111111111111")
	feesAccountIDStr := getEnv("FEES_ACCOUNT_ID", "22222222-2222-2222-2222-222222222222")
	platformAccountID, err := uuid.Parse(platformAccountIDStr)
	if err != nil {
		slog.Error("invalid PLATFORM_ACCOUNT_ID UUID", "error", err)
		os.Exit(1)
	}
	feesAccountID, err := uuid.Parse(feesAccountIDStr)
	if err != nil {
		slog.Error("invalid FEES_ACCOUNT_ID UUID", "error", err)
		os.Exit(1)
	}

	merchantFeeBps, _ := strconv.ParseInt(getEnv("MERCHANT_FEE_BPS", "500"), 10, 64)
	enableScheduler := getEnv("ENABLE_SCHEDULER", "true") == "true"
	enableWebhookDispatcher := getEnv("ENABLE_WEBHOOK_DISPATCHER", "true") == "true"

	slog.Info("starting ledgerly bnpl service",
		"port", port,
		"ledger_service_url", ledgerURL,
		"platform_account_id", platformAccountID,
		"fees_account_id", feesAccountID,
		"merchant_fee_bps", merchantFeeBps,
		"enable_scheduler", enableScheduler,
		"enable_webhook_dispatcher", enableWebhookDispatcher,
	)

	// Connect to PostgreSQL
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
		slog.Error("failed to apply database migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("database migrations applied successfully")

	// Repositories
	orderRepo := postgres.NewOrderRepository(db)
	attemptRepo := postgres.NewPaymentAttemptRepository(db)
	outboxRepo := postgres.NewOutboxRepository(db)

	// Gateway and Ledger client
	gw := gateway.NewSimulator()
	ledgerClient := ledgerclient.NewClient(ledgerURL, 5*time.Second)

	// Application service
	orderSvc := service.NewOrderService(
		orderRepo,
		attemptRepo,
		outboxRepo,
		gw,
		ledgerClient,
		platformAccountID,
		feesAccountID,
		merchantFeeBps,
	)

	// HTTP Server
	server := apihttp.NewServer(orderSvc, db)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      server.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()

	// Optional background Installment Scheduler loop
	if enableScheduler {
		sched := scheduler.NewInstallmentScheduler(
			scheduler.DefaultConfig(),
			orderRepo,
			attemptRepo,
			outboxRepo,
			gw,
			ledgerClient,
		)
		go runSchedulerLoop(appCtx, sched, 15*time.Second)
	}

	// Optional background Webhook Dispatcher loop
	if enableWebhookDispatcher {
		webhookSecret := getEnv("WEBHOOK_SECRET", "ledgerly_whsec_secret_default_key")
		dispatcher := webhook.NewDispatcher(webhook.DefaultConfig(webhookSecret), outboxRepo)
		go runWebhookDispatcherLoop(appCtx, dispatcher, 5*time.Second)
	}

	// Graceful shutdown handling
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
	cancelApp()

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := httpServer.Shutdown(ctxShutdown); err != nil {
		slog.Error("http server graceful shutdown failed", "error", err)
	}

	slog.Info("bnpl service stopped cleanly")
}

func runSchedulerLoop(ctx context.Context, sched *scheduler.InstallmentScheduler, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("installment scheduler worker running", "interval", interval.String())
	for {
		select {
		case <-ctx.Done():
			slog.Info("installment scheduler worker stopped")
			return
		case t := <-ticker.C:
			count, err := sched.ProcessDueInstallments(ctx, t)
			if err != nil {
				slog.Error("scheduler run failed", "error", err)
			} else if count > 0 {
				slog.Info("processed due installments", "count", count)
			}
		}
	}
}

func runWebhookDispatcherLoop(ctx context.Context, dispatcher *webhook.Dispatcher, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("webhook dispatcher worker running", "interval", interval.String())
	for {
		select {
		case <-ctx.Done():
			slog.Info("webhook dispatcher worker stopped")
			return
		case t := <-ticker.C:
			count, err := dispatcher.ProcessPendingEvents(ctx, t)
			if err != nil {
				slog.Error("webhook dispatcher run failed", "error", err)
			} else if count > 0 {
				slog.Info("dispatched webhook events", "count", count)
			}
		}
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
