package dao

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestGetPromptTemplateByKeyMissingDoesNotWriteRecordNotFound 验证启动期 seed 查询缺失内置模板时不会污染 stdout。
func TestGetPromptTemplateByKeyMissingDoesNotWriteRecordNotFound(t *testing.T) {
	var logs bytes.Buffer
	db, err := gorm.Open(sqlite.Open(testSQLitePath(t)), &gorm.Config{
		Logger: logger.New(log.New(&logs, "", 0), logger.Config{LogLevel: logger.Warn}),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.PromptTemplate{}); err != nil {
		t.Fatalf("migrate prompt template: %v", err)
	}
	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	_, ok, err := store.GetPromptTemplateByKey(context.Background(), "builtin_stock_full")
	if err != nil {
		t.Fatalf("get prompt template by key: %v", err)
	}
	if ok {
		t.Fatal("expected missing builtin prompt")
	}
	if strings.Contains(logs.String(), "record not found") {
		t.Fatalf("missing builtin lookup must not write GORM record-not-found logs: %q", logs.String())
	}
}
