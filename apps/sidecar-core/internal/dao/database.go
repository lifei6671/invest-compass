package dao

import (
	"context"
	"fmt"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config 描述本地 SQLite 连接需要的最小配置。
type Config struct {
	Path string
}

// Open 使用 GORM 打开 SQLite，并通过 PingContext 验证当前数据库可访问。
func Open(ctx context.Context, config Config) (*gorm.DB, error) {
	if config.Path == "" {
		return nil, fmt.Errorf("dao sqlite path is required")
	}

	db, err := gorm.Open(sqlite.Open(config.Path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite with gorm: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("resolve sqlite db handle: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}

	return db, nil
}

// Migrate 创建或补齐首版本地 SQLite schema，供 Go core 启动时执行。
func Migrate(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("dao db is required")
	}

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.AutoMigrate(
			&model.Stock{},
			&model.Watchlist{},
			&model.Quote{},
			&model.Kline{},
			&model.NewsItem{},
			&model.AIConfig{},
			&model.PromptTemplate{},
			&model.AnalysisReport{},
			&model.Task{},
			&model.TaskEvent{},
			&model.Setting{},
		)
	})
	if err != nil {
		return fmt.Errorf("migrate sqlite schema: %w", err)
	}
	return nil
}
