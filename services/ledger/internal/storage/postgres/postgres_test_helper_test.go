package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/stretchr/testify/require"
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
		port := uint32(54333)
		var err error
		tempDataDir, err = os.MkdirTemp("", "embedded-pg-shared")
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

	// Load migration SQL
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

// setupTestDB returns a DB connection scoped to an isolated PostgreSQL schema for the test.
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	schemaName := fmt.Sprintf("test_schema_%d_%d", time.Now().Unix(), atomic.AddInt64(&schemaCounter, 1))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create schema and set search path
	_, err := sharedDB.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s;", schemaName))
	require.NoError(t, err)

	// Open connection with search_path set to the isolated schema
	// In PostgreSQL URL or connection string:
	// We can execute migration within schema
	prepSQL := fmt.Sprintf("SET search_path TO %s, public;\n%s", schemaName, migrationSQL)
	err = postgres.ExecuteMigrationScript(ctx, sharedDB, prepSQL)
	require.NoError(t, err)

	// Return a wrapper connection that runs queries with search_path set to schemaName
	// Or we can create a sub-DB or pool with options search_path=schemaName
	// For pgx/sql.Open:
	origConnStr := os.Getenv("TEST_DATABASE_URL")
	if origConnStr == "" {
		origConnStr = os.Getenv("DATABASE_URL")
	}
	if origConnStr == "" {
		origConnStr = "postgres://ledgerly_test:ledgerly_test_pw@localhost:54333/ledgerly_test_db?sslmode=disable"
	}

	sep := "?"
	if strings.Contains(origConnStr, "?") {
		sep = "&"
	}
	schemaConnStr := fmt.Sprintf("%s%ssearch_path=%s,public", origConnStr, sep, schemaName)

	testDB, err := postgres.Open(postgres.Config{
		URL:             schemaConnStr,
		MaxOpenConns:    25,
		MaxIdleConns:    25,
		ConnMaxLifetime: 2 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	})
	require.NoError(t, err)

	teardown := func() {
		_ = testDB.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = sharedDB.ExecContext(cleanupCtx, fmt.Sprintf("DROP SCHEMA %s CASCADE;", schemaName))
	}

	return testDB, teardown
}
