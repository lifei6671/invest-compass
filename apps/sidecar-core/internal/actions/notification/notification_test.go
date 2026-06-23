package notification

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestListReturnsPagedNotifications 验证通知列表路由会透传分页和未读过滤。
func TestListReturnsPagedNotifications(t *testing.T) {
	store := &fakeNotificationStore{
		notifications: []model.Notification{{
			ID:        1,
			Type:      "task",
			Level:     "success",
			Title:     "分析完成",
			Content:   "任务已完成",
			Route:     "/tasks/1",
			CreatedAt: time.Date(2026, 6, 23, 9, 0, 0, 0, time.UTC),
		}},
		total: 1,
	}
	recorder := performNotificationAction(t, "/api/notifications/list", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
	}, `{"unread_only":true,"limit":10,"offset":5}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if !store.unreadOnly || store.limit != 10 || store.offset != 5 {
		t.Fatalf("unexpected store list args unread=%v limit=%d offset=%d", store.unreadOnly, store.limit, store.offset)
	}
	data := decodeActionData(t, recorder.Body.Bytes())
	if data["total"].(float64) != 1 {
		t.Fatalf("unexpected total: %#v", data)
	}
}

// TestUnreadCountReturnsCount 验证未读数量路由返回角标所需 count。
func TestUnreadCountReturnsCount(t *testing.T) {
	recorder := performNotificationAction(t, "/api/notifications/unread-count", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    &fakeNotificationStore{unreadCount: 2},
	}, `{}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	data := decodeActionData(t, recorder.Body.Bytes())
	if data["count"].(float64) != 2 {
		t.Fatalf("unexpected unread count: %#v", data)
	}
}

// TestMarkReadRejectsEmptyIDs 验证标记已读路由拒绝空 ID，防止误操作全量通知。
func TestMarkReadRejectsEmptyIDs(t *testing.T) {
	store := &fakeNotificationStore{}
	recorder := performNotificationAction(t, "/api/notifications/mark-read", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
	}, `{"ids":[]}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	if len(store.markReadIDs) > 0 {
		t.Fatalf("empty ids must not call store, got %v", store.markReadIDs)
	}
}

// TestClearReadCallsStore 验证清理已读只调用通知表清理。
func TestClearReadCallsStore(t *testing.T) {
	store := &fakeNotificationStore{}
	recorder := performNotificationAction(t, "/api/notifications/clear-read", Config{
		Security: httpx.SecurityConfig{Token: "test-token", Ready: true},
		Store:    store,
	}, `{}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if !store.clearReadCalled {
		t.Fatal("expected clear read to call store")
	}
}

// performNotificationAction 执行指定通知路由并返回响应记录器。
func performNotificationAction(t *testing.T, path string, config Config, body string) *httptest.ResponseRecorder {
	t.Helper()
	var handler http.HandlerFunc
	for _, route := range Routes(config) {
		if route.Path == path {
			handler = route.Handler
			break
		}
	}
	if handler == nil {
		t.Fatalf("route %s not found", path)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set(httpx.TokenHeader, "test-token")
	handler.ServeHTTP(recorder, request)
	return recorder
}

// decodeActionData 解析统一响应 envelope 的 data 字段。
func decodeActionData(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var envelope struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode response: %v body=%s", err, string(body))
	}
	if envelope.Code != 0 {
		t.Fatalf("unexpected code %d body=%s", envelope.Code, string(body))
	}
	return envelope.Data
}

type fakeNotificationStore struct {
	notifications     []model.Notification
	created           []model.Notification
	total             int64
	unreadOnly        bool
	limit             int
	offset            int
	unreadCount       int64
	markReadIDs       []int64
	markAllReadCalled bool
	clearReadCalled   bool
}

// CreateNotification 记录创建通知调用，满足 action Store 接口。
func (store *fakeNotificationStore) CreateNotification(_ context.Context, notification *model.Notification) error {
	if notification != nil {
		store.created = append(store.created, *notification)
	}
	return nil
}

// FindNotificationBySource 返回测试中未预置的通知来源查询结果。
func (store *fakeNotificationStore) FindNotificationBySource(context.Context, string, string, string) (model.Notification, bool, error) {
	return model.Notification{}, false, nil
}

// GetAnalysisReportByTaskID 返回测试中未预置的报告查询结果。
func (store *fakeNotificationStore) GetAnalysisReportByTaskID(context.Context, string) (model.AnalysisReport, bool, error) {
	return model.AnalysisReport{}, false, nil
}

// ListNotifications 记录分页参数并返回预置通知。
func (store *fakeNotificationStore) ListNotifications(_ context.Context, unreadOnly bool, limit int, offset int) ([]model.Notification, int64, error) {
	store.unreadOnly = unreadOnly
	store.limit = limit
	store.offset = offset
	return store.notifications, store.total, nil
}

// CountUnreadNotifications 返回预置未读数量。
func (store *fakeNotificationStore) CountUnreadNotifications(context.Context) (int64, error) {
	return store.unreadCount, nil
}

// MarkNotificationsRead 记录已读通知 ID。
func (store *fakeNotificationStore) MarkNotificationsRead(_ context.Context, ids []int64) error {
	store.markReadIDs = append([]int64(nil), ids...)
	return nil
}

// MarkAllNotificationsRead 记录批量已读调用。
func (store *fakeNotificationStore) MarkAllNotificationsRead(context.Context) error {
	store.markAllReadCalled = true
	return nil
}

// ClearReadNotifications 记录清理已读通知调用。
func (store *fakeNotificationStore) ClearReadNotifications(context.Context) error {
	store.clearReadCalled = true
	return nil
}
