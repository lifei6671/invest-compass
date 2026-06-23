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
		"task_log_entries",
		"task_error_diagnoses",
		"settings",
		"scheduler_jobs",
		"scheduler_runs",
		"ingestion_watermarks",
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
	assertColumns(t, db, "task_log_entries", "task_id", "request_id", "trace_id", "ts", "level", "module", "stage", "payload_json")
	assertColumns(t, db, "task_error_diagnoses", "task_id", "error_code", "error_stage", "summary", "causes_json", "suggestions_json")
	assertColumns(t, db, "scheduler_jobs", "cron_type", "catchup_enabled", "catchup_max_days", "deleted_at")
	assertColumns(t, db, "scheduler_runs", "cron_type", "data_type", "period", "params_json", "run_key", "trigger_type", "target_date", "scope_key")
	assertColumns(t, db, "ingestion_watermarks", "data_type", "scope_key", "provider", "period", "last_trade_date")
	if db.Migrator().HasColumn("scheduler_jobs", "type") {
		t.Fatal("scheduler_jobs must use cron_type column instead of reserved type column")
	}
}

// TestTaskLogMigration 验证结构化任务日志专题新增表能随标准迁移一次性创建。
func TestTaskLogMigration(t *testing.T) {
	db, err := Open(context.Background(), Config{Path: testSQLitePath(t)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	for _, table := range []string{"task_log_entries", "task_error_diagnoses"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected %s to exist", table)
		}
	}
	assertColumns(t, db, "task_log_entries", "task_id", "level", "module", "stage", "message", "payload_json")
	assertColumns(t, db, "task_error_diagnoses", "task_id", "summary", "retryable", "source_log_id")
}

// TestSearchMigrationCreatesIndexSchema 验证范围搜索专题的普通表和 FTS5 虚表随迁移创建。
func TestSearchMigrationCreatesIndexSchema(t *testing.T) {
	db, err := Open(context.Background(), Config{Path: testSQLitePath(t)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	for _, table := range []string{
		"stock_aliases",
		"stock_pinyin_overrides",
		"search_documents",
		"search_index_batches",
		"search_index_state",
		"search_index_jobs",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected search table %s to exist", table)
		}
		assertColumns(t, db, table, "created_at", "updated_at")
	}
	assertColumns(t, db, "stocks", "full_name", "pinyin_full", "pinyin_initials", "search_name", "search_version", "indexed_at")
	assertColumns(t, db, "search_documents", "batch_id", "doc_uid", "doc_type", "ref_table", "ref_id", "indexed_at", "index_version", "deleted_at")
	assertColumns(t, db, "search_index_batches", "batch_id", "scope", "status", "dictionary_hash", "error_message")
	assertColumns(t, db, "search_index_jobs", "doc_type", "ref_id", "operation", "status", "attempts", "last_error")

	for _, table := range []string{"stock_search_fts", "search_documents_fts"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected FTS5 table %s to exist", table)
		}
	}
	if err := db.Exec(`INSERT INTO stock_search_fts(batch_id, symbol, market, exchange, code, name_index, pinyin_initials) VALUES (?, ?, ?, ?, ?, ?, ?)`, "batch-1", "CN:SH:600519", "CN", "SH", "600519", "贵州 茅台 贵州茅台", "gzmt").Error; err != nil {
		t.Fatalf("insert stock_search_fts row: %v", err)
	}
	var count int
	if err := db.Raw(`SELECT count(*) FROM stock_search_fts WHERE stock_search_fts MATCH ?`, "茅台").Scan(&count).Error; err != nil {
		t.Fatalf("query stock_search_fts: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one stock_search_fts match, got %d", count)
	}
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

// TestProbeSQLiteFTS5ReportsAvailable 验证当前 sidecar 测试二进制启用了 SQLite FTS5。
func TestProbeSQLiteFTS5ReportsAvailable(t *testing.T) {
	db, err := Open(context.Background(), Config{Path: testSQLitePath(t)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	status, err := ProbeSQLiteFTS5(context.Background(), db)
	if err != nil {
		t.Fatalf("probe sqlite fts5: %v", err)
	}
	if status != SQLiteFTS5Available {
		t.Fatalf("expected sqlite fts5 status %q, got %q", SQLiteFTS5Available, status)
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

// TestBackupBeforeMigrationBackupCanBeRestoredAndMigrated 验证迁移前备份可以作为恢复点重新打开并保留用户数据。
func TestBackupBeforeMigrationBackupCanBeRestoredAndMigrated(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "invest-compass.sqlite3")
	db, err := Open(ctx, Config{Path: dbPath})
	if err != nil {
		t.Fatalf("open sqlite file: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate sqlite file: %v", err)
	}
	if err := db.Exec(
		`INSERT INTO settings(key, value, created_at, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"workspace.default_path",
		"/Users/demo/InvestCompass",
	).Error; err != nil {
		t.Fatalf("insert user setting before backup: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve sqlite db handle: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sqlite before backup: %v", err)
	}

	result, err := BackupBeforeMigration(ctx, BackupConfig{
		Path:      dbPath,
		BackupDir: filepath.Join(tempDir, "backups"),
		Now:       time.Date(2026, 6, 18, 8, 9, 10, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("backup sqlite before restore rehearsal: %v", err)
	}
	if !result.Created || len(result.Files) != 1 {
		t.Fatalf("expected one sqlite backup file, got %+v", result)
	}

	backupContent, err := os.ReadFile(result.Files[0])
	if err != nil {
		t.Fatalf("read sqlite backup file: %v", err)
	}
	restoredPath := filepath.Join(tempDir, "restored.sqlite3")
	if err := os.WriteFile(restoredPath, backupContent, 0o600); err != nil {
		t.Fatalf("write restored sqlite file: %v", err)
	}
	restoredDB, err := Open(ctx, Config{Path: restoredPath})
	if err != nil {
		t.Fatalf("open restored sqlite file: %v", err)
	}
	if err := Migrate(ctx, restoredDB); err != nil {
		t.Fatalf("migrate restored sqlite file: %v", err)
	}
	var restoredValue string
	if err := restoredDB.Raw(`SELECT value FROM settings WHERE key = ?`, "workspace.default_path").Scan(&restoredValue).Error; err != nil {
		t.Fatalf("read restored user setting: %v", err)
	}
	if restoredValue != "/Users/demo/InvestCompass" {
		t.Fatalf("expected restored user setting to survive backup restore, got %q", restoredValue)
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

// TestBackupBeforeMigrationSkipsExistingDailyBackup 验证同一数据库当天已有自动备份时不再重复生成。
func TestBackupBeforeMigrationSkipsExistingDailyBackup(t *testing.T) {
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

	result, err := BackupBeforeMigration(context.Background(), BackupConfig{
		Path:      dbPath,
		BackupDir: backupDir,
		Now:       time.Date(2026, 6, 18, 8, 9, 10, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("backup should skip existing daily backup without error: %v", err)
	}
	if result.Created || len(result.Files) != 0 {
		t.Fatalf("expected existing daily backup to skip new files, got %+v", result)
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

	insertSchedulerRun := `INSERT INTO scheduler_runs(job_id, cron_type, data_type, period, run_key, trigger_type, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
	if err := db.Exec(insertSchedulerRun, 1, "cn_a_share_quote_refresh", "quote", "", "job-1:2026-06-19:missed_today", "missed_today", "queued").Error; err != nil {
		t.Fatalf("insert first scheduler run: %v", err)
	}
	if err := db.Exec(insertSchedulerRun, 1, "cn_a_share_quote_refresh", "quote", "", "job-1:2026-06-19:missed_today", "missed_today", "queued").Error; err == nil {
		t.Fatal("expected duplicate scheduler run_key to fail")
	}

	insertWatermark := `INSERT INTO ingestion_watermarks(data_type, scope_key, provider, period, created_at, updated_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
	if err := db.Exec(insertWatermark, "quote", "CN:SH:600519", "sina_tencent", "").Error; err != nil {
		t.Fatalf("insert first ingestion watermark: %v", err)
	}
	if err := db.Exec(insertWatermark, "quote", "CN:SH:600519", "sina_tencent", "").Error; err == nil {
		t.Fatal("expected duplicate ingestion watermark key to fail")
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
