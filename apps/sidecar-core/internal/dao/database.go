package dao

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config 描述本地 SQLite 连接需要的最小配置。
type Config struct {
	Path string
}

// BackupConfig 描述迁移前 SQLite 文件备份所需的输入。
type BackupConfig struct {
	Path      string
	BackupDir string
	Now       time.Time
}

// BackupResult 描述迁移前备份是否生成以及生成的文件列表。
type BackupResult struct {
	Created bool
	Files   []string
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

// BackupBeforeMigration 在 schema 迁移前备份已有 SQLite 文件；首次启动和内存库会跳过。
func BackupBeforeMigration(ctx context.Context, config BackupConfig) (BackupResult, error) {
	if err := ctx.Err(); err != nil {
		return BackupResult{}, err
	}
	if strings.TrimSpace(config.Path) == "" {
		return BackupResult{}, fmt.Errorf("dao sqlite path is required")
	}
	if strings.HasPrefix(config.Path, "file:") {
		return BackupResult{}, nil
	}
	info, err := os.Stat(config.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return BackupResult{}, nil
		}
		return BackupResult{}, fmt.Errorf("stat sqlite before migration backup: %w", err)
	}
	if info.IsDir() {
		return BackupResult{}, fmt.Errorf("sqlite path is directory")
	}
	if info.Size() == 0 {
		return BackupResult{}, nil
	}
	if strings.TrimSpace(config.BackupDir) == "" {
		return BackupResult{}, fmt.Errorf("backup dir is required")
	}
	backupTime := config.Now.UTC()
	if backupTime.IsZero() {
		backupTime = time.Now().UTC()
	}
	if err := os.MkdirAll(config.BackupDir, 0o700); err != nil {
		return BackupResult{}, fmt.Errorf("create sqlite backup dir: %w", err)
	}

	sources := sqliteBackupSources(config.Path)
	files := make([]string, 0, len(sources))
	for _, source := range sources {
		if err := ctx.Err(); err != nil {
			return BackupResult{}, err
		}
		sourceInfo, err := os.Stat(source)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return BackupResult{}, fmt.Errorf("stat sqlite backup source: %w", err)
		}
		if sourceInfo.IsDir() || sourceInfo.Size() == 0 {
			continue
		}
		target := sqliteBackupPath(config.BackupDir, source, backupTime)
		if err := copyFileNoOverwrite(source, target); err != nil {
			return BackupResult{}, err
		}
		files = append(files, target)
	}
	return BackupResult{Created: len(files) > 0, Files: files}, nil
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
			&model.SchedulerJob{},
			&model.SchedulerRun{},
			&model.IngestionWatermark{},
		)
	})
	if err != nil {
		return fmt.Errorf("migrate sqlite schema: %w", err)
	}
	return nil
}

// sqliteBackupSources 返回 SQLite 主库和可能存在的 WAL/SHM 伴随文件。
func sqliteBackupSources(path string) []string {
	return []string{path, path + "-wal", path + "-shm"}
}

// sqliteBackupPath 使用 UTC 时间生成不会混淆平台路径的备份文件名。
func sqliteBackupPath(backupDir string, source string, backupTime time.Time) string {
	baseName := filepath.Base(source)
	extension := filepath.Ext(baseName)
	name := strings.TrimSuffix(baseName, extension)
	timestamp := backupTime.Format("20060102T150405000000000Z")
	return filepath.Join(backupDir, fmt.Sprintf("%s-%s%s.bak", name, timestamp, extension))
}

// copyFileNoOverwrite 复制文件并拒绝覆盖已有备份，避免破坏用户可恢复点。
func copyFileNoOverwrite(source string, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open sqlite backup source: %w", err)
	}
	defer input.Close()

	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create sqlite backup target: %w", err)
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return fmt.Errorf("copy sqlite backup: %w", err)
	}
	if err := output.Sync(); err != nil {
		return fmt.Errorf("sync sqlite backup: %w", err)
	}
	return nil
}
