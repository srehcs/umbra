package testutil

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DefaultSchemaFiles = []string{
	"0001_init.sql",
	"0002_add_request_id.sql",
	"0003_add_receipt_indexes.sql",
	"0004_add_receipt_search_indexes.sql",
	"0005_add_receipt_search_text.sql",
	"0006_add_receipt_canonical.sql",
}

// ApplySchemaForTests applies the control-plane schema for integration tests.
func ApplySchemaForTests(t *testing.T, pool *pgxpool.Pool) error {
	t.Helper()
	for _, name := range DefaultSchemaFiles {
		content, err := os.ReadFile(schemaPath(name))
		if err != nil {
			return err
		}
		stmts := strings.Split(string(content), ";")
		for _, stmt := range stmts {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if _, err := pool.Exec(context.Background(), stmt); err != nil {
				return err
			}
		}
	}
	return nil
}

func schemaPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	return filepath.Join(root, "migrations", name)
}
