package http_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apihttp "gitlab.com/steph-ano/ledgerly/services/bnpl/internal/api/http"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/domain"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/gateway"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/ledgerclient"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/service"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/storage/postgres"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/migrations"
)

var (
	sharedDB       *sql.DB
	schemaCounter  int64
	embeddedPGInst *embeddedpostgres.EmbeddedPostgres
	tempDataDir    string
)

func TestMain(m *testing.M) {
	connStr := os.Getenv("TEST_DATABASE_URL")
	if connStr == "" {
		connStr = os.Getenv("DATABASE_URL")
	}

	if connStr == "" {
		port := uint32(54356)
		var err error
		tempDataDir, err = os.MkdirTemp("", "embedded-pg-bnpl-http")
		if err != nil {
			fmt.Printf("failed to create temp dir: %v\n", err)
			os.Exit(1)
		}

		cfg := embeddedpostgres.DefaultConfig().
			Username("ledgerly_test").
			Password("ledgerly_test_pw").
			Database("ledgerly_test_db").
			Port(port).
			RuntimePath(tempDataDir).
			DataPath(filepath.Join(tempDataDir, "data"))

		embeddedPGInst = embeddedpostgres.NewDatabase(cfg)
		if err := embeddedPGInst.Start(); err != nil {
			fmt.Printf("failed to start embedded postgres on port %d: %v\n", port, err)
			_ = os.RemoveAll(tempDataDir)
			os.Exit(1)
		}
		connStr = fmt.Sprintf("postgres://ledgerly_test:ledgerly_test_pw@localhost:%d/ledgerly_test_db?sslmode=disable", port)
	}

	var err error
	sharedDB, err = postgres.Open(postgres.Config{
		URL:             connStr,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
	})
	if err != nil {
		fmt.Printf("failed to connect to shared db: %v\n", err)
		if embeddedPGInst != nil {
			_ = embeddedPGInst.Stop()
			_ = os.RemoveAll(tempDataDir)
		}
		os.Exit(1)
	}

	code := m.Run()

	_ = sharedDB.Close()
	if embeddedPGInst != nil {
		_ = embeddedPGInst.Stop()
		_ = os.RemoveAll(tempDataDir)
	}
	os.Exit(code)
}

func setupHTTPServer(t *testing.T) (*apihttp.Server, *sql.DB, func()) {
	t.Helper()

	val := atomic.AddInt64(&schemaCounter, 1)
	schemaName := fmt.Sprintf("test_http_schema_%d_%d", time.Now().UnixNano()%1000000, val)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := sharedDB.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s;", schemaName))
	require.NoError(t, err)

	_, err = sharedDB.ExecContext(ctx, fmt.Sprintf("SET search_path TO %s, public;", schemaName))
	require.NoError(t, err)

	cleanMigration := migrations.UpSQL
	cleanMigration = strings.ReplaceAll(cleanMigration, "public.update_updated_at_column", fmt.Sprintf("%s.update_updated_at_column", schemaName))

	_, err = sharedDB.ExecContext(ctx, cleanMigration)
	require.NoError(t, err)

	orderRepo := postgres.NewOrderRepository(sharedDB)
	attemptRepo := postgres.NewPaymentAttemptRepository(sharedDB)
	outboxRepo := postgres.NewOutboxRepository(sharedDB)
	gw := gateway.NewSimulator()

	// Mock ledger server
	mockLedger := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"transaction":{"id":"` + uuid.New().String() + `","status":"posted"}}`))
	}))

	ledgerClient := ledgerclient.NewClient(mockLedger.URL, 5*time.Second)

	platformID := uuid.New()
	feesID := uuid.New()
	svc := service.NewOrderService(
		orderRepo,
		attemptRepo,
		outboxRepo,
		gw,
		ledgerClient,
		platformID,
		feesID,
		500, // 5%
	)

	server := apihttp.NewServer(svc, sharedDB)

	teardown := func() {
		mockLedger.Close()
		tdCtx, tdCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer tdCancel()
		_, _ = sharedDB.ExecContext(tdCtx, fmt.Sprintf("DROP SCHEMA %s CASCADE;", schemaName))
	}

	return server, sharedDB, teardown
}

func TestServer_HealthzAndReadyz(t *testing.T) {
	server, _, teardown := setupHTTPServer(t)
	defer teardown()

	handler := server.Handler()

	// Healthz
	reqHealth := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rrHealth := httptest.NewRecorder()
	handler.ServeHTTP(rrHealth, reqHealth)

	assert.Equal(t, http.StatusOK, rrHealth.Code)
	assert.Contains(t, rrHealth.Body.String(), `"status":"ok"`)

	// Readyz
	reqReady := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rrReady := httptest.NewRecorder()
	handler.ServeHTTP(rrReady, reqReady)

	assert.Equal(t, http.StatusOK, rrReady.Code)
	assert.Contains(t, rrReady.Body.String(), `"status":"ready"`)
}

func TestServer_CreateOrder_Success(t *testing.T) {
	server, _, teardown := setupHTTPServer(t)
	defer teardown()

	handler := server.Handler()

	custID := uuid.New()
	merchID := uuid.New()

	payload := map[string]any{
		"client_id":            "client_test_e2e_1",
		"customer_account_id":  custID.String(),
		"merchant_account_id":  merchID.String(),
		"total_amount":         10000,
		"currency":             "USD",
		"payment_method_token": "pm_card_visa",
		"merchant_webhook_url": "https://merchant.example.com/webhook",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)

	var created domain.Order
	err := json.Unmarshal(rr.Body.Bytes(), &created)
	require.NoError(t, err)

	assert.Equal(t, domain.OrderStatusActive, created.Status)
	assert.Equal(t, int64(10000), created.TotalAmount)
	assert.Len(t, created.Installments, 4)
	assert.Equal(t, domain.InstallmentStatusPaid, created.Installments[0].Status)
	assert.Equal(t, domain.InstallmentStatusPending, created.Installments[1].Status)

	// Fetch via GET /v1/orders/{id}
	reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/orders/%s", created.ID.String()), nil)
	rrGet := httptest.NewRecorder()
	handler.ServeHTTP(rrGet, reqGet)

	require.Equal(t, http.StatusOK, rrGet.Code)
	var fetched domain.Order
	err = json.Unmarshal(rrGet.Body.Bytes(), &fetched)
	require.NoError(t, err)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, int64(10000), fetched.TotalAmount)
}

func TestServer_CreateOrder_DeclinedDownPayment(t *testing.T) {
	server, _, teardown := setupHTTPServer(t)
	defer teardown()

	handler := server.Handler()

	payload := map[string]any{
		"client_id":            "client_test_declined",
		"customer_account_id":  uuid.New().String(),
		"merchant_account_id":  uuid.New().String(),
		"total_amount":         12000,
		"currency":             "USD",
		"payment_method_token": "pm_card_insufficient_funds",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusPaymentRequired, rr.Code)
	assert.Contains(t, rr.Body.String(), "DOWN_PAYMENT_DECLINED")
}

func TestServer_CreateOrder_InvalidAmount(t *testing.T) {
	server, _, teardown := setupHTTPServer(t)
	defer teardown()

	handler := server.Handler()

	payload := map[string]any{
		"client_id":            "client_invalid",
		"customer_account_id":  uuid.New().String(),
		"merchant_account_id":  uuid.New().String(),
		"total_amount":         -100,
		"currency":             "USD",
		"payment_method_token": "pm_card_visa",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "BAD_REQUEST")
}

func TestServer_GetOrder_NotFound(t *testing.T) {
	server, _, teardown := setupHTTPServer(t)
	defer teardown()

	handler := server.Handler()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/orders/%s", uuid.New().String()), nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "NOT_FOUND")
}

func TestServer_PayInstallment_Lifecycle(t *testing.T) {
	server, _, teardown := setupHTTPServer(t)
	defer teardown()

	handler := server.Handler()

	// 1. Create order
	payload := map[string]any{
		"client_id":            "client_pay_inst",
		"customer_account_id":  uuid.New().String(),
		"merchant_account_id":  uuid.New().String(),
		"total_amount":         10000,
		"currency":             "USD",
		"payment_method_token": "pm_card_visa",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	var order domain.Order
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &order))

	inst2 := order.Installments[1]

	// 2. Pay installment #2 manually via API
	payPayload := map[string]any{
		"payment_method_token": "pm_card_mastercard",
	}
	payBody, _ := json.Marshal(payPayload)
	reqPay := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/installments/%s/pay", inst2.ID.String()), bytes.NewReader(payBody))
	reqPay.Header.Set("Content-Type", "application/json")
	rrPay := httptest.NewRecorder()
	handler.ServeHTTP(rrPay, reqPay)

	assert.Equal(t, http.StatusOK, rrPay.Code)
	var paidInst domain.Installment
	require.NoError(t, json.Unmarshal(rrPay.Body.Bytes(), &paidInst))
	assert.Equal(t, domain.InstallmentStatusPaid, paidInst.Status)

	// 3. Attempt paying again -> should fail with ALREADY_PAID
	rrPayAgain := httptest.NewRecorder()
	reqPayAgain := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/installments/%s/pay", inst2.ID.String()), bytes.NewReader(payBody))
	handler.ServeHTTP(rrPayAgain, reqPayAgain)

	assert.Equal(t, http.StatusBadRequest, rrPayAgain.Code)
	assert.Contains(t, rrPayAgain.Body.String(), "ALREADY_PAID")
}
