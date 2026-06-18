package dao

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

// TestOpenRequiresPath 验证 dao 入口必须显式传入数据库路径。
func TestOpenRequiresPath(t *testing.T) {
	db, err := Open(context.Background(), Config{})

	if err == nil {
		t.Fatal("expected error for empty dao sqlite path")
	}
	if db != nil {
		t.Fatalf("expected nil db for empty dao sqlite path, got %#v", db)
	}
}

// TestOpenConnectsSQLiteWithGORM 验证 dao 入口通过 GORM 打开 SQLite。
func TestOpenConnectsSQLiteWithGORM(t *testing.T) {
	db, err := Open(context.Background(), Config{Path: "file::memory:?cache=shared"})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve sqlite db handle: %v", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.PingContext(context.Background()); err != nil {
		t.Fatalf("ping sqlite db: %v", err)
	}
}

// TestMigrateCreatesInitialSchema 验证首版 SQLite schema 能在空库完整创建。
func TestMigrateCreatesInitialSchema(t *testing.T) {
	db, err := Open(context.Background(), Config{Path: "file::memory:?cache=shared"})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate empty database: %v", err)
	}

	expectedTables := []string{
		"stocks",
		"watchlists",
		"quotes",
		"klines",
		"news_items",
		"ai_configs",
		"prompt_templates",
		"analysis_reports",
		"tasks",
		"task_events",
		"settings",
	}
	for _, table := range expectedTables {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected table %s to exist", table)
		}
		assertColumns(t, db, table, "created_at", "updated_at")
	}

	for _, table := range []string{"watchlists", "news_items", "ai_configs", "prompt_templates", "analysis_reports"} {
		assertColumns(t, db, table, "deleted_at")
	}
	assertColumns(t, db, "ai_configs", "api_key_ref", "masked_api_key", "has_api_key")
}

// TestMigrateIsIdempotent 验证重复迁移不会破坏已有数据库。
func TestMigrateIsIdempotent(t *testing.T) {
	db, err := Open(context.Background(), Config{Path: "file::memory:?cache=shared"})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	if !db.Migrator().HasTable("task_events") {
		t.Fatal("expected task_events table to remain after repeated migration")
	}
}

// TestInitialSchemaEnforcesCoreUniqueConstraints 验证首版关键唯一约束真实生效。
func TestInitialSchemaEnforcesCoreUniqueConstraints(t *testing.T) {
	db, err := Open(context.Background(), Config{Path: "file::memory:?cache=shared"})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	if err := db.Exec(`INSERT INTO watchlists(symbol, created_at, updated_at) VALUES (?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, "US:AAPL").Error; err != nil {
		t.Fatalf("insert first active watchlist: %v", err)
	}
	if err := db.Exec(`INSERT INTO watchlists(symbol, created_at, updated_at) VALUES (?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, "US:AAPL").Error; err == nil {
		t.Fatal("expected duplicate active watchlist symbol to fail")
	}
	if err := db.Exec(`UPDATE watchlists SET deleted_at = CURRENT_TIMESTAMP WHERE symbol = ?`, "US:AAPL").Error; err != nil {
		t.Fatalf("soft delete active watchlist: %v", err)
	}
	if err := db.Exec(`INSERT INTO watchlists(symbol, created_at, updated_at) VALUES (?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, "US:AAPL").Error; err != nil {
		t.Fatalf("insert watchlist after soft delete: %v", err)
	}

	insertKline := `INSERT INTO klines(symbol, period, adjust, trade_date, created_at, updated_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
	if err := db.Exec(insertKline, "US:AAPL", "day", "none", "2026-06-18").Error; err != nil {
		t.Fatalf("insert first kline: %v", err)
	}
	if err := db.Exec(insertKline, "US:AAPL", "day", "none", "2026-06-18").Error; err == nil {
		t.Fatal("expected duplicate kline key to fail")
	}
}

// assertColumns 验证迁移后的表包含指定列，避免 schema 缺列时静默通过。
func assertColumns(t *testing.T, db *gorm.DB, table string, columns ...string) {
	t.Helper()
	for _, column := range columns {
		if !db.Migrator().HasColumn(table, column) {
			t.Fatalf("expected table %s to have column %s", table, column)
		}
	}
}
