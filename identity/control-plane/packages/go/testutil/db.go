package testutil

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	stor "github.com/umbra-labs/agent-identity-control-plane/packages/go/storage"
)

// RequireTestDatabase ensures integration tests only run against a dedicated test DB.
// Override by setting UMBRA_ALLOW_NON_TEST_DB=1 for local/dev use.
func RequireTestDatabase(t *testing.T, dsn string) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("UMBRA_ALLOW_NON_TEST_DB")) != "" {
		return
	}
	dbName := dbNameFromDSN(dsn)
	lowered := strings.ToLower(dbName)
	if dbName == "" || (!strings.Contains(lowered, "test") && !strings.Contains(lowered, "ci")) {
		t.Fatalf("UMBRA_TEST_DATABASE_URL must point to a dedicated test db (got %q); set UMBRA_ALLOW_NON_TEST_DB=1 to override", dbName)
	}
}

// ConnectIsolatedTestDB provisions a per-test schema and returns a DB scoped to it.
func ConnectIsolatedTestDB(t *testing.T, dsn string) (*stor.DB, func()) {
	t.Helper()
	RequireTestDatabase(t, dsn)

	schema := newSchemaName()
	basePool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("base db connect failed: %v", err)
	}
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := basePool.Exec(context.Background(), fmt.Sprintf("CREATE SCHEMA %s", ident)); err != nil {
		basePool.Close()
		t.Fatalf("create schema failed: %v", err)
	}

	scopedDSN := withSearchPath(dsn, schema)
	db, err := stor.Connect(context.Background(), scopedDSN)
	if err != nil {
		_ = dropSchema(basePool, ident)
		basePool.Close()
		t.Fatalf("scoped db connect failed: %v", err)
	}

	cleanup := func() {
		db.Close()
		_ = dropSchema(basePool, ident)
		basePool.Close()
	}
	return db, cleanup
}

func dbNameFromDSN(dsn string) string {
	if strings.Contains(dsn, "://") {
		if parsed, err := url.Parse(dsn); err == nil {
			dbName := strings.TrimPrefix(parsed.Path, "/")
			if dbName == "" {
				dbName = parsed.Query().Get("dbname")
			}
			return dbName
		}
		return ""
	}
	for _, field := range strings.Fields(dsn) {
		if strings.HasPrefix(field, "dbname=") {
			return strings.TrimPrefix(field, "dbname=")
		}
	}
	return ""
}

func withSearchPath(dsn, schema string) string {
	if strings.Contains(dsn, "://") {
		parsed, err := url.Parse(dsn)
		if err != nil {
			return dsn
		}
		q := parsed.Query()
		q.Set("search_path", schema)
		parsed.RawQuery = q.Encode()
		return parsed.String()
	}
	return strings.TrimSpace(dsn) + " search_path=" + schema
}

func newSchemaName() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("umbra_test_%d_%d", time.Now().UnixNano(), r.Intn(100000))
}

func dropSchema(pool *pgxpool.Pool, ident string) error {
	_, err := pool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", ident))
	return err
}
