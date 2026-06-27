package notification

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestListNormalizesPaginationAndRedactsText 验证通知列表会限制分页并在响应边界脱敏。
func TestListNormalizesPaginationAndRedactsText(t *testing.T) {
	createdAt := time.Date(2026, 6, 23, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{
		notifications: []model.Notification{{
			ID:        7,
			Type:      "task",
			Level:     "success",
			Title:     "任务完成",
			Content:   "Authorization: Bearer sk-test-secret",
			Route:     "/tasks/7",
			CreatedAt: createdAt,
		}},
		total: 1,
	}
	result, err := NewService(store).List(context.Background(), ListRequest{UnreadOnly: true, Offset: 2})
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if store.unreadOnly != true || store.limit != DefaultListLimit || store.offset != 2 {
		t.Fatalf("unexpected list arguments unread=%v limit=%d offset=%d", store.unreadOnly, store.limit, store.offset)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("unexpected list result: %+v", result)
	}
	if result.Items[0].Content == "Authorization: Bearer sk-test-secret" {
		t.Fatal("notification content must be redacted before returning to frontend")
	}
	if result.Items[0].CreatedAt != "2026-06-23T09:00:00Z" {
		t.Fatalf("unexpected created_at: %q", result.Items[0].CreatedAt)
	}
}

// TestListRejectsInvalidPagination 验证通知列表拒绝负 offset 和过大 limit。
func TestListRejectsInvalidPagination(t *testing.T) {
	service := NewService(&fakeStore{})
	if !errors.Is(mustListErr(t, service, ListRequest{Limit: MaxListLimit + 1}), ErrInvalidRequest) {
		t.Fatal("too large limit should return ErrInvalidRequest")
	}
	if !errors.Is(mustListErr(t, service, ListRequest{Offset: -1}), ErrInvalidRequest) {
		t.Fatal("negative offset should return ErrInvalidRequest")
	}
}

// TestMarkReadRejectsEmptyAndInvalidIDs 验证标记已读必须显式指定正数 ID。
func TestMarkReadRejectsEmptyAndInvalidIDs(t *testing.T) {
	service := NewService(&fakeStore{})
	if !errors.Is(service.MarkRead(context.Background(), nil), ErrInvalidRequest) {
		t.Fatal("empty ids should return ErrInvalidRequest")
	}
	if !errors.Is(service.MarkRead(context.Background(), []int64{1, 0}), ErrInvalidRequest) {
		t.Fatal("non-positive ids should return ErrInvalidRequest")
	}
}

// TestUnreadAndClearDelegateToStore 验证未读计数和清理已读只委托通知表操作。
func TestUnreadAndClearDelegateToStore(t *testing.T) {
	store := &fakeStore{unreadCount: 3}
	service := NewService(store)
	count, err := service.UnreadCount(context.Background())
	if err != nil {
		t.Fatalf("unread count: %v", err)
	}
	if count.Count != 3 {
		t.Fatalf("unexpected count: %+v", count)
	}
	if err := service.MarkAllRead(context.Background()); err != nil {
		t.Fatalf("mark all read: %v", err)
	}
	if err := service.ClearRead(context.Background()); err != nil {
		t.Fatalf("clear read: %v", err)
	}
	if !store.markAllReadCalled || !store.clearReadCalled {
		t.Fatalf("expected mark all and clear read delegation, got mark=%v clear=%v", store.markAllReadCalled, store.clearReadCalled)
	}
}

// TestNotifyTaskSuccessRoutesToReportWhenReportExists 验证分析任务成功通知优先跳转到报告详情。
func TestNotifyTaskSuccessRoutesToReportWhenReportExists(t *testing.T) {
	store := &fakeStore{
		report: model.AnalysisReport{
			ID:     88,
			TaskID: "task-success",
			Title:  "浦发银行分析",
		},
		hasReport: true,
	}
	task := model.Task{
		ID:     "task-success",
		Status: "SUCCESS",
		Title:  "CN:SH:600522 stock_full",
	}
	created, err := NewService(store).NotifyTaskTerminal(context.Background(), task)
	if err != nil {
		t.Fatalf("notify task success: %v", err)
	}
	if !created {
		t.Fatal("expected notification to be created")
	}
	if store.created == nil {
		t.Fatal("expected notification model")
	}
	if store.created.Type != TypeTaskSuccess || store.created.Level != LevelSuccess {
		t.Fatalf("unexpected notification type/level: %+v", store.created)
	}
	if store.created.SourceType != SourceTypeTask || store.created.SourceID != "task-success" {
		t.Fatalf("unexpected source: %+v", store.created)
	}
	if store.created.Route != "/reports/88" {
		t.Fatalf("success notification should route to report detail, got %q", store.created.Route)
	}
	if strings.Contains(store.created.Content, "stock_full") || store.created.Content != "CN:SH:600522 个股综合分析报告已生成，可点击查看。" {
		t.Fatalf("unexpected friendly content: %q", store.created.Content)
	}
}

// TestNotifyTaskFailedRoutesToTasks 验证任务失败通知统一跳转任务历史页，并脱敏错误信息。
func TestNotifyTaskFailedRoutesToTasks(t *testing.T) {
	store := &fakeStore{}
	task := model.Task{
		ID:           "task-failed",
		Status:       "FAILED",
		Title:        "DeepSeek 分析",
		ErrorMessage: "Authorization: Bearer sk-real-secret",
	}
	created, err := NewService(store).NotifyTaskTerminal(context.Background(), task)
	if err != nil {
		t.Fatalf("notify task failed: %v", err)
	}
	if !created {
		t.Fatal("expected failed notification to be created")
	}
	if store.created.Type != TypeTaskFailed || store.created.Level != LevelError {
		t.Fatalf("unexpected notification type/level: %+v", store.created)
	}
	if store.created.Route != "/tasks" {
		t.Fatalf("failed notification should route to tasks, got %q", store.created.Route)
	}
	if store.created.Content == task.ErrorMessage {
		t.Fatal("task failed notification content must be redacted")
	}
}

// TestNotifyTaskTerminalSkipsDuplicateSource 验证同一个任务终态不会重复生成多条通知。
func TestNotifyTaskTerminalSkipsDuplicateSource(t *testing.T) {
	store := &fakeStore{existingNotification: true}
	task := model.Task{ID: "task-repeat", Status: "SUCCESS", Title: "重复任务"}
	created, err := NewService(store).NotifyTaskTerminal(context.Background(), task)
	if err != nil {
		t.Fatalf("notify duplicate task: %v", err)
	}
	if created {
		t.Fatal("duplicate source should not create another notification")
	}
	if store.created != nil {
		t.Fatalf("unexpected created notification: %+v", store.created)
	}
}

// TestNotifyProviderErrorRoutesToSettings 验证 Provider 异常通知跳转基础设置页。
func TestNotifyProviderErrorRoutesToSettings(t *testing.T) {
	store := &fakeStore{}
	created, err := NewService(store).NotifyProviderError(context.Background(), ProviderErrorInput{
		Name:      "market",
		Source:    "sina",
		LastError: "Cookie: token=secret",
	})
	if err != nil {
		t.Fatalf("notify provider error: %v", err)
	}
	if !created {
		t.Fatal("expected provider notification to be created")
	}
	if store.created.Type != TypeProviderError || store.created.Level != LevelWarning {
		t.Fatalf("unexpected notification type/level: %+v", store.created)
	}
	if store.created.SourceType != SourceTypeProvider || store.created.SourceID != "market:sina" {
		t.Fatalf("unexpected provider source: %+v", store.created)
	}
	if store.created.Route != "/settings" {
		t.Fatalf("provider notification should route to settings, got %q", store.created.Route)
	}
	if store.created.Content == "Cookie: token=secret" {
		t.Fatal("provider notification content must be redacted")
	}
}

// mustListErr 执行列表请求并返回错误，测试中用于聚焦非法参数断言。
func mustListErr(t *testing.T, service Service, request ListRequest) error {
	t.Helper()
	_, err := service.List(context.Background(), request)
	if err == nil {
		t.Fatal("expected list error")
	}
	return err
}

type fakeStore struct {
	notifications        []model.Notification
	total                int64
	unreadOnly           bool
	limit                int
	offset               int
	unreadCount          int64
	markReadIDs          []int64
	markAllReadCalled    bool
	clearReadCalled      bool
	created              *model.Notification
	report               model.AnalysisReport
	hasReport            bool
	existingNotification bool
}

// CreateNotification 记录新建通知。
func (store *fakeStore) CreateNotification(_ context.Context, notification *model.Notification) error {
	copied := *notification
	store.created = &copied
	return nil
}

// FindNotificationBySource 返回同源通知是否已经存在。
func (store *fakeStore) FindNotificationBySource(context.Context, string, string, string) (model.Notification, bool, error) {
	return model.Notification{}, store.existingNotification, nil
}

// GetAnalysisReportByTaskID 返回预置报告，用于任务成功通知跳转。
func (store *fakeStore) GetAnalysisReportByTaskID(context.Context, string) (model.AnalysisReport, bool, error) {
	return store.report, store.hasReport, nil
}

// ListNotifications 记录分页参数并返回预置通知。
func (store *fakeStore) ListNotifications(_ context.Context, unreadOnly bool, limit int, offset int) ([]model.Notification, int64, error) {
	store.unreadOnly = unreadOnly
	store.limit = limit
	store.offset = offset
	return store.notifications, store.total, nil
}

// CountUnreadNotifications 返回预置未读数量。
func (store *fakeStore) CountUnreadNotifications(context.Context) (int64, error) {
	return store.unreadCount, nil
}

// MarkNotificationsRead 记录已读通知 ID。
func (store *fakeStore) MarkNotificationsRead(_ context.Context, ids []int64) error {
	store.markReadIDs = append([]int64(nil), ids...)
	return nil
}

// MarkAllNotificationsRead 记录批量已读调用。
func (store *fakeStore) MarkAllNotificationsRead(context.Context) error {
	store.markAllReadCalled = true
	return nil
}

// ClearReadNotifications 记录清理已读通知调用。
func (store *fakeStore) ClearReadNotifications(context.Context) error {
	store.clearReadCalled = true
	return nil
}
