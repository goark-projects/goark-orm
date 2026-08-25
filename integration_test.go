package orm

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func TestIntegrationDatabaseSmoke_whenConfigured_shouldOpenAndPing(t *testing.T) {
	driverName := os.Getenv("GOARK_ORM_INTEGRATION_DRIVER")
	dsn := os.Getenv("GOARK_ORM_INTEGRATION_DSN")
	if driverName == "" || dsn == "" {
		t.Skip("set GOARK_ORM_INTEGRATION_DRIVER and GOARK_ORM_INTEGRATION_DSN to run database smoke test")
	}
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping database failed: %v", err)
	}
}
