package dao

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	db, err := Open(context.Background(), Config{Path: testSQLitePath(t)})
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
	db, err := Open(context.Background(), Config{Path: testSQLitePath(t)})
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
	db, err := Open(context.Background(), Config{Path: testSQLitePath(t)})
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

// TestBackupBeforeMigrationCopiesExistingSQLiteFile 验证迁移前备份会复制已有用户数据库文件。
func TestBackupBeforeMigrationCopiesExistingSQLiteFile(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "invest-compass.sqlite3")
	if err := os.WriteFile(dbPath, []byte("user database"), 0o600); err != nil {
		t.Fatalf("write sqlite file: %v", err)
	}
	backupTime := time.Date(2026, 6, 18, 8, 9, 10, 0, time.UTC)

	result, err := BackupBeforeMigration(context.Background(), BackupConfig{
		Path:      dbPath,
		BackupDir: filepath.Join(tempDir, "backups"),
		Now:       backupTime,
	})
	if err != nil {
		t.Fatalf("backup sqlite before migration: %v", err)
	}

	expectedPath := filepath.Join(tempDir, "backups", "invest-compass-20260618T080910000000000Z.sqlite3.bak")
	if !result.Created || len(result.Files) != 1 || result.Files[0] != expectedPath {
		t.Fatalf("unexpected backup result: %+v", result)
	}
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read backup file: %v", err)
	}
	if string(content) != "user database" {
		t.Fatalf("unexpected backup content: %q", string(content))
	}
}

// TestBackupBeforeMigrationSkipsMissingOrMemoryDatabase 验证首次启动或内存库不会生成无意义备份。
func TestBackupBeforeMigrationSkipsMissingOrMemoryDatabase(t *testing.T) {
	tempDir := t.TempDir()
	for _, path := range []string{
		filepath.Join(tempDir, "missing.sqlite3"),
		"file:memory?mode=memory&cache=shared",
	} {
		result, err := BackupBeforeMigration(context.Background(), BackupConfig{
			Path:      path,
			BackupDir: filepath.Join(tempDir, "backups"),
			Now:       time.Date(2026, 6, 18, 8, 9, 10, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("backup should skip %s without error: %v", path, err)
		}
		if result.Created || len(result.Files) != 0 {
			t.Fatalf("expected no backup for %s, got %+v", path, result)
		}
	}
}

// TestBackupBeforeMigrationRefusesOverwrite 验证备份文件重名时直接失败，避免覆盖已有用户备份。
func TestBackupBeforeMigrationRefusesOverwrite(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "invest-compass.sqlite3")
	backupDir := filepath.Join(tempDir, "backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatalf("create backup dir: %v", err)
	}
	if err := os.WriteFile(dbPath, []byte("current database"), 0o600); err != nil {
		t.Fatalf("write sqlite file: %v", err)
	}
	existing := filepath.Join(backupDir, "invest-compass-20260618T080910000000000Z.sqlite3.bak")
	if err := os.WriteFile(existing, []byte("existing backup"), 0o600); err != nil {
		t.Fatalf("write existing backup: %v", err)
	}

	_, err := BackupBeforeMigration(context.Background(), BackupConfig{
		Path:      dbPath,
		BackupDir: backupDir,
		Now:       time.Date(2026, 6, 18, 8, 9, 10, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected backup overwrite to fail")
	}
	content, readErr := os.ReadFile(existing)
	if readErr != nil {
		t.Fatalf("read existing backup: %v", readErr)
	}
	if string(content) != "existing backup" {
		t.Fatalf("existing backup was overwritten: %q", string(content))
	}
}

// TestInitialSchemaEnforcesCoreUniqueConstraints 验证首版关键唯一约束真实生效。
func TestInitialSchemaEnforcesCoreUniqueConstraints(t *testing.T) {
	db, err := Open(context.Background(), Config{Path: testSQLitePath(t)})
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

// testSQLitePath 为每个测试创建隔离的共享内存库，避免跨测试数据污染。
func testSQLitePath(t *testing.T) string {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	return fmt.Sprintf("file:%s?mode=memory&cache=shared", name)
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
