package tasklog

import (
	"context"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
)

// TestRetentionManagerAppliesDefaultPolicy 验证默认策略符合 30 天、单任务 5000、全库 50 万条。
func TestRetentionManagerAppliesDefaultPolicy(t *testing.T) {
	now := time.Date(2025, 5, 20, 15, 30, 0, 0, time.UTC)
	store := &recordingRetentionStore{}
	manager, err := NewRetentionManager(RetentionConfig{
		Store: store,
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("new retention manager: %v", err)
	}

	if _, err := manager.Apply(context.Background()); err != nil {
		t.Fatalf("apply retention: %v", err)
	}

	wantCutoff := now.AddDate(0, 0, -DefaultRetentionDays)
	if !store.options.Before.Equal(wantCutoff) {
		t.Fatalf("unexpected cutoff: got %s want %s", store.options.Before, wantCutoff)
	}
	if store.options.PerTaskLimit != DefaultMaxEntriesPerTask {
		t.Fatalf("unexpected per task limit: %+v", store.options)
	}
	if store.options.TotalLimit != DefaultMaxTotalEntries {
		t.Fatalf("unexpected total limit: %+v", store.options)
	}
}

// TestRetentionManagerUsesCustomPolicy 验证自定义保留策略会原样传递给 DAO。
func TestRetentionManagerUsesCustomPolicy(t *testing.T) {
	now := time.Date(2025, 5, 20, 15, 30, 0, 0, time.UTC)
	store := &recordingRetentionStore{}
	manager, err := NewRetentionManager(RetentionConfig{
		Store:             store,
		Now:               func() time.Time { return now },
		RetentionDays:     7,
		MaxEntriesPerTask: 100,
		MaxTotalEntries:   1000,
	})
	if err != nil {
		t.Fatalf("new retention manager: %v", err)
	}

	if _, err := manager.Apply(context.Background()); err != nil {
		t.Fatalf("apply retention: %v", err)
	}

	if !store.options.Before.Equal(now.AddDate(0, 0, -7)) ||
		store.options.PerTaskLimit != 100 ||
		store.options.TotalLimit != 1000 {
		t.Fatalf("unexpected custom options: %+v", store.options)
	}
}

type recordingRetentionStore struct {
	options dao.TaskLogRetentionOptions
}

// PruneTaskLogs 记录 retention service 下发的裁剪参数。
func (store *recordingRetentionStore) PruneTaskLogs(_ context.Context, options dao.TaskLogRetentionOptions) (dao.TaskLogRetentionResult, error) {
	store.options = options
	return dao.TaskLogRetentionResult{}, nil
}
