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

	apihttp "gitlab.com/steph-ano/ledgerly/services/ledger/internal/api/http"
	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/domain"
	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/service"
	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/storage/postgres"
)

var (
	sharedDB       *sql.DB
	migrationSQL   string
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
		port := uint32(54344)
		var err error
		tempDataDir, err = os.MkdirTemp("", "embedded-pg-http-shared")
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
			fmt.Printf("failed to start embedded postgres: %v\n", err)
			_ = os.RemoveAll(tempDataDir)
			os.Exit(1)
		}
		connStr = fmt.Sprintf("postgres://ledgerly_test:ledgerly_test_pw@localhost:%d/ledgerly_test_db?sslmode=disable", port)
	}

	var err error
	sharedDB, err = postgres.Open(postgres.Config{
		URL:             connStr,
		MaxOpenConns:    50,
		MaxIdleConns:    50,
		ConnMaxLifetime: 10 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	})
	if err != nil {
		fmt.Printf("failed to connect to shared test db: %v\n", err)
		cleanup()
		os.Exit(1)
	}

	candidates := []string{
		filepath.Join("..", "..", "migrations", "000001_init_ledger_schema.up.sql"),
		filepath.Join("..", "..", "..", "migrations", "000001_init_ledger_schema.up.sql"),
		filepath.Join("migrations", "000001_init_ledger_schema.up.sql"),
	}
	for _, c := range candidates {
		if b, err := os.ReadFile(c); err == nil {
			migrationSQL = string(b)
			break
		}
	}
	if migrationSQL == "" {
		fmt.Println("failed to find migration file 000001_init_ledger_schema.up.sql")
		cleanup()
		os.Exit(1)
	}

	code := m.Run()

	cleanup()
	os.Exit(code)
}

func cleanup() {
	if sharedDB != nil {
		_ = sharedDB.Close()
	}
	if embeddedPGInst != nil {
		_ = embeddedPGInst.Stop()
	}
	if tempDataDir != "" {
		_ = os.RemoveAll(tempDataDir)
	}
}

func setupTestServer(t *testing.T) (*apihttp.Server, func()) {
	t.Helper()

	schemaName := fmt.Sprintf("http_test_schema_%d_%d", time.Now().Unix(), atomic.AddInt64(&schemaCounter, 1))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := sharedDB.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s;", schemaName))
	require.NoError(t, err)

	prepSQL := fmt.Sprintf("SET search_path TO %s, public;\n%s", schemaName, migrationSQL)
	err = postgres.ExecuteMigrationScript(ctx, sharedDB, prepSQL)
	require.NoError(t, err)

	origConnStr := os.Getenv("TEST_DATABASE_URL")
	if origConnStr == "" {
		origConnStr = os.Getenv("DATABASE_URL")
	}
	if origConnStr == "" {
		origConnStr = "postgres://ledgerly_test:ledgerly_test_pw@localhost:54344/ledgerly_test_db?sslmode=disable"
	}

	sep := "?"
	if strings.Contains(origConnStr, "?") {
		sep = "&"
	}
	schemaConnStr := fmt.Sprintf("%s%ssearch_path=%s,public", origConnStr, sep, schemaName)

	testDB, err := postgres.Open(postgres.Config{
		URL:             schemaConnStr,
		MaxOpenConns:    20,
		MaxIdleConns:    20,
		ConnMaxLifetime: 2 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	})
	require.NoError(t, err)

	accRepo := postgres.NewAccountRepository(testDB)
	txRepo := postgres.NewTransactionRepository(testDB)
	svc := service.NewLedgerService(accRepo, txRepo, 0)
	srv := apihttp.NewServer(svc, testDB)

	teardown := func() {
		_ = testDB.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = sharedDB.ExecContext(cleanupCtx, fmt.Sprintf("DROP SCHEMA %s CASCADE;", schemaName))
	}

	return srv, teardown
}

func TestHTTP_HealthAndReadiness(t *testing.T) {
	srv, teardown := setupTestServer(t)
	defer teardown()

	handler := srv.Handler()

	// GET /healthz
	reqHealth := httptest.NewRequest("GET", "/healthz", nil)
	recHealth := httptest.NewRecorder()
	handler.ServeHTTP(recHealth, reqHealth)
	assert.Equal(t, http.StatusOK, recHealth.Code)
	assert.Contains(t, recHealth.Body.String(), `"status":"ok"`)

	// GET /readyz
	reqReady := httptest.NewRequest("GET", "/readyz", nil)
	recReady := httptest.NewRecorder()
	handler.ServeHTTP(recReady, reqReady)
	assert.Equal(t, http.StatusOK, recReady.Code)
	assert.Contains(t, recReady.Body.String(), `"status":"ready"`)
}

func TestHTTP_AccountEndpoints(t *testing.T) {
	srv, teardown := setupTestServer(t)
	defer teardown()

	handler := srv.Handler()

	// 1. Create account POST /v1/accounts
	accPayload := map[string]any{
		"client_id": "test_merchant_client",
		"type":      "merchant",
		"currency":  "USD",
	}
	body, _ := json.Marshal(accPayload)
	req := httptest.NewRequest("POST", "/v1/accounts", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var created domain.Account
	err := json.Unmarshal(rec.Body.Bytes(), &created)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, domain.AccountTypeMerchant, created.Type)

	// 2. Get account GET /v1/accounts/{id}
	reqGet := httptest.NewRequest("GET", fmt.Sprintf("/v1/accounts/%s", created.ID), nil)
	recGet := httptest.NewRecorder()
	handler.ServeHTTP(recGet, reqGet)
	assert.Equal(t, http.StatusOK, recGet.Code)

	// 3. Get balance GET /v1/accounts/{id}/balance
	reqBal := httptest.NewRequest("GET", fmt.Sprintf("/v1/accounts/%s/balance", created.ID), nil)
	recBal := httptest.NewRecorder()
	handler.ServeHTTP(recBal, reqBal)
	assert.Equal(t, http.StatusOK, recBal.Code)
	var bal domain.AccountBalance
	err = json.Unmarshal(recBal.Body.Bytes(), &bal)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal.Debits)
	assert.Equal(t, int64(0), bal.Credits)
}

func TestHTTP_TransactionAndIdempotencyEndpoints(t *testing.T) {
	srv, teardown := setupTestServer(t)
	defer teardown()

	handler := srv.Handler()

	// Setup two accounts: Customer and Merchant
	createAcc := func(accType domain.AccountType) uuid.UUID {
		body, _ := json.Marshal(map[string]any{
			"client_id": "http_client_1",
			"type":      accType,
			"currency":  "USD",
		})
		req := httptest.NewRequest("POST", "/v1/accounts", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusCreated, rec.Code)
		var acc domain.Account
		_ = json.Unmarshal(rec.Body.Bytes(), &acc)
		return acc.ID
	}

	custID := createAcc(domain.AccountTypeCustomer)
	merchID := createAcc(domain.AccountTypeMerchant)

	txPayload := map[string]any{
		"description": "BNPL First Installment",
		"entries": []map[string]any{
			{"account_id": custID.String(), "amount": 2500, "direction": "DEBIT", "currency": "USD"},
			{"account_id": merchID.String(), "amount": 2500, "direction": "CREDIT", "currency": "USD"},
		},
	}
	body, _ := json.Marshal(txPayload)

	// 1. Initial Transaction: 201 Created
	req1 := httptest.NewRequest("POST", "/v1/transactions", bytes.NewReader(body))
	req1.Header.Set("Idempotency-Key", "http_idem_key_100")
	req1.Header.Set("X-Client-ID", "http_client_1")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	assert.Equal(t, http.StatusCreated, rec1.Code)
	var tx1 domain.Transaction
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &tx1))
	assert.NotEqual(t, uuid.Nil, tx1.ID)

	// 2. Retry with same Idempotency-Key and payload: 200 OK with same transaction
	req2 := httptest.NewRequest("POST", "/v1/transactions", bytes.NewReader(body))
	req2.Header.Set("Idempotency-Key", "http_idem_key_100")
	req2.Header.Set("X-Client-ID", "http_client_1")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusOK, rec2.Code)
	var tx2 domain.Transaction
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &tx2))
	assert.Equal(t, tx1.ID, tx2.ID)

	// 3. Retry with same Idempotency-Key but different payload: 409 Conflict
	mismatchPayload := map[string]any{
		"description": "Different Description Mismatch",
		"entries": []map[string]any{
			{"account_id": custID.String(), "amount": 2500, "direction": "DEBIT", "currency": "USD"},
			{"account_id": merchID.String(), "amount": 2500, "direction": "CREDIT", "currency": "USD"},
		},
	}
	mismatchBody, _ := json.Marshal(mismatchPayload)
	reqMismatch := httptest.NewRequest("POST", "/v1/transactions", bytes.NewReader(mismatchBody))
	reqMismatch.Header.Set("Idempotency-Key", "http_idem_key_100")
	reqMismatch.Header.Set("X-Client-ID", "http_client_1")
	recMismatch := httptest.NewRecorder()
	handler.ServeHTTP(recMismatch, reqMismatch)

	assert.Equal(t, http.StatusConflict, recMismatch.Code)
	assert.Contains(t, recMismatch.Body.String(), "IDEMPOTENCY_PAYLOAD_MISMATCH")

	// 4. Reversal: POST /v1/transactions/{id}/reverse
	revPayload := map[string]any{
		"description": "Customer Refund",
	}
	revBody, _ := json.Marshal(revPayload)
	reqRev := httptest.NewRequest("POST", fmt.Sprintf("/v1/transactions/%s/reverse", tx1.ID), bytes.NewReader(revBody))
	reqRev.Header.Set("Idempotency-Key", "rev_idem_key_200")
	reqRev.Header.Set("X-Client-ID", "http_client_1")
	recRev := httptest.NewRecorder()
	handler.ServeHTTP(recRev, reqRev)

	assert.Equal(t, http.StatusCreated, recRev.Code)
	var revTx domain.Transaction
	require.NoError(t, json.Unmarshal(recRev.Body.Bytes(), &revTx))
	assert.Equal(t, &tx1.ID, revTx.ReversalOf)

	// 5. Entries List: GET /v1/accounts/{id}/entries
	reqEntries := httptest.NewRequest("GET", fmt.Sprintf("/v1/accounts/%s/entries", custID), nil)
	recEntries := httptest.NewRecorder()
	handler.ServeHTTP(recEntries, reqEntries)

	assert.Equal(t, http.StatusOK, recEntries.Code)
	assert.Contains(t, recEntries.Body.String(), `"entries"`)
}
