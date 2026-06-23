package notification

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
)

const (
	// DefaultListLimit 是通知浮层默认分页大小，避免首屏读取过多历史通知。
	DefaultListLimit = 20
	// MaxListLimit 是通知列表单次读取上限，防止前端误传无界分页。
	MaxListLimit = 50
	// TypeTaskSuccess 表示任务成功通知。
	TypeTaskSuccess = "task_success"
	// TypeTaskFailed 表示任务失败通知。
	TypeTaskFailed = "task_failed"
	// TypeProviderError 表示 Provider 异常通知。
	TypeProviderError = "provider_error"
	// LevelSuccess 表示成功级别通知。
	LevelSuccess = "success"
	// LevelError 表示错误级别通知。
	LevelError = "error"
	// LevelWarning 表示警告级别通知。
	LevelWarning = "warning"
	// SourceTypeTask 表示通知来源是任务。
	SourceTypeTask = "task"
	// SourceTypeProvider 表示通知来源是 Provider。
	SourceTypeProvider = "provider"
)

// ErrInvalidRequest 表示通知 API 收到了无法执行的参数。
var ErrInvalidRequest = errors.New("invalid notification request")

// Store 是通知 service 依赖的持久化边界。
type Store interface {
	CreateNotification(ctx context.Context, notification *model.Notification) error
	FindNotificationBySource(ctx context.Context, sourceType string, sourceID string, notificationType string) (model.Notification, bool, error)
	GetAnalysisReportByTaskID(ctx context.Context, taskID string) (model.AnalysisReport, bool, error)
	ListNotifications(ctx context.Context, unreadOnly bool, limit int, offset int) ([]model.Notification, int64, error)
	CountUnreadNotifications(ctx context.Context) (int64, error)
	MarkNotificationsRead(ctx context.Context, ids []int64) error
	MarkAllNotificationsRead(ctx context.Context) error
	ClearReadNotifications(ctx context.Context) error
}

// Service 管理应用内通知的查询和已读状态。
type Service struct {
	store Store
}

// ListRequest 描述通知列表分页请求。
type ListRequest struct {
	UnreadOnly bool `json:"unread_only"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
}

// ListResult 是通知列表 API 返回的分页结果。
type ListResult struct {
	Items  []Item `json:"items"`
	Total  int64  `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// UnreadCountResult 是通知角标使用的未读数量。
type UnreadCountResult struct {
	Count int64 `json:"count"`
}

// Item 是前端可展示的脱敏通知摘要。
type Item struct {
	ID         int64   `json:"id"`
	Type       string  `json:"type"`
	Level      string  `json:"level"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	SourceType string  `json:"source_type"`
	SourceID   string  `json:"source_id"`
	Route      string  `json:"route"`
	IsRead     bool    `json:"is_read"`
	CreatedAt  string  `json:"created_at"`
	ReadAt     *string `json:"read_at"`
}

// ProviderErrorInput 是生成 Provider 异常通知所需的最小脱敏输入。
type ProviderErrorInput struct {
	Name      string
	Source    string
	LastError string
}

// NewService 构造应用内通知 service。
func NewService(store Store) Service {
	return Service{store: store}
}

// NotifyTaskTerminal 根据任务终态生成应用内通知；非终态或重复来源不会写入新记录。
func (service Service) NotifyTaskTerminal(ctx context.Context, task model.Task) (bool, error) {
	if service.store == nil {
		return false, fmt.Errorf("notification store is required")
	}
	notificationType := ""
	level := ""
	title := ""
	content := ""
	route := ""
	switch task.Status {
	case "SUCCESS":
		notificationType = TypeTaskSuccess
		level = LevelSuccess
		title = "任务完成"
		content = notificationTaskTitle(task) + " 已完成"
		route = service.taskSuccessRoute(ctx, task.ID)
	case "FAILED":
		notificationType = TypeTaskFailed
		level = LevelError
		title = "任务失败"
		content = strings.TrimSpace(notificationTaskTitle(task) + " 执行失败")
		if strings.TrimSpace(task.ErrorMessage) != "" {
			content = content + "：" + task.ErrorMessage
		}
		route = "/tasks"
	default:
		return false, nil
	}
	created, err := service.createOnce(ctx, model.Notification{
		Type:       notificationType,
		Level:      level,
		Title:      title,
		Content:    logger.RedactText(content),
		SourceType: SourceTypeTask,
		SourceID:   task.ID,
		Route:      route,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	})
	return created, err
}

// NotifyProviderError 根据 Provider 异常生成应用内通知；同源同类型通知只生成一次。
func (service Service) NotifyProviderError(ctx context.Context, input ProviderErrorInput) (bool, error) {
	if service.store == nil {
		return false, fmt.Errorf("notification store is required")
	}
	name := strings.TrimSpace(input.Name)
	source := strings.TrimSpace(input.Source)
	if name == "" {
		name = "provider"
	}
	sourceID := name
	if source != "" {
		sourceID = name + ":" + source
	}
	content := strings.TrimSpace(input.LastError)
	if content == "" {
		content = "数据源状态异常"
	}
	now := time.Now().UTC()
	return service.createOnce(ctx, model.Notification{
		Type:       TypeProviderError,
		Level:      LevelWarning,
		Title:      "数据源异常",
		Content:    logger.RedactText(content),
		SourceType: SourceTypeProvider,
		SourceID:   sourceID,
		Route:      "/settings",
		CreatedAt:  now,
		UpdatedAt:  now,
	})
}

// List 返回分页通知列表，所有文本字段在响应前会再次脱敏。
func (service Service) List(ctx context.Context, request ListRequest) (ListResult, error) {
	if service.store == nil {
		return ListResult{}, fmt.Errorf("notification store is required")
	}
	limit, offset, err := normalizePagination(request.Limit, request.Offset)
	if err != nil {
		return ListResult{}, err
	}
	notifications, total, err := service.store.ListNotifications(ctx, request.UnreadOnly, limit, offset)
	if err != nil {
		return ListResult{}, err
	}
	items := make([]Item, 0, len(notifications))
	for _, notification := range notifications {
		items = append(items, itemFromModel(notification))
	}
	return ListResult{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

// UnreadCount 返回未读通知数量，用于全局右上角角标。
func (service Service) UnreadCount(ctx context.Context) (UnreadCountResult, error) {
	if service.store == nil {
		return UnreadCountResult{}, fmt.Errorf("notification store is required")
	}
	count, err := service.store.CountUnreadNotifications(ctx)
	if err != nil {
		return UnreadCountResult{}, err
	}
	return UnreadCountResult{Count: count}, nil
}

// MarkRead 将指定通知标记为已读。
func (service Service) MarkRead(ctx context.Context, ids []int64) error {
	if service.store == nil {
		return fmt.Errorf("notification store is required")
	}
	if len(ids) == 0 {
		return ErrInvalidRequest
	}
	for _, id := range ids {
		if id <= 0 {
			return ErrInvalidRequest
		}
	}
	return service.store.MarkNotificationsRead(ctx, ids)
}

// MarkAllRead 将所有未读通知标记为已读。
func (service Service) MarkAllRead(ctx context.Context) error {
	if service.store == nil {
		return fmt.Errorf("notification store is required")
	}
	return service.store.MarkAllNotificationsRead(ctx)
}

// ClearRead 清理已读通知记录，不影响通知来源对象。
func (service Service) ClearRead(ctx context.Context) error {
	if service.store == nil {
		return fmt.Errorf("notification store is required")
	}
	return service.store.ClearReadNotifications(ctx)
}

// createOnce 按 source_type/source_id/type 做幂等写入，避免任务终态重复通知。
func (service Service) createOnce(ctx context.Context, notification model.Notification) (bool, error) {
	if strings.TrimSpace(notification.SourceType) == "" || strings.TrimSpace(notification.SourceID) == "" || strings.TrimSpace(notification.Type) == "" {
		return false, ErrInvalidRequest
	}
	_, exists, err := service.store.FindNotificationBySource(ctx, notification.SourceType, notification.SourceID, notification.Type)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	if err := service.store.CreateNotification(ctx, &notification); err != nil {
		return false, err
	}
	return true, nil
}

// taskSuccessRoute 返回任务成功通知的目标路由，有报告时优先打开报告详情。
func (service Service) taskSuccessRoute(ctx context.Context, taskID string) string {
	report, ok, err := service.store.GetAnalysisReportByTaskID(ctx, taskID)
	if err != nil || !ok || report.ID <= 0 {
		return "/tasks"
	}
	return "/reports/" + strconv.FormatInt(report.ID, 10)
}

// notificationTaskTitle 返回任务标题，缺失时用 task_id 保证通知仍可识别来源。
func notificationTaskTitle(task model.Task) string {
	if title := strings.TrimSpace(task.Title); title != "" {
		return title
	}
	return strings.TrimSpace(task.ID)
}

// normalizePagination 统一分页边界，limit 为空时使用默认值。
func normalizePagination(limit int, offset int) (int, int, error) {
	if offset < 0 {
		return 0, 0, ErrInvalidRequest
	}
	if limit == 0 {
		limit = DefaultListLimit
	}
	if limit < 0 || limit > MaxListLimit {
		return 0, 0, ErrInvalidRequest
	}
	return limit, offset, nil
}

// itemFromModel 转换持久化模型为前端响应，同时对展示文本做二次脱敏。
func itemFromModel(notification model.Notification) Item {
	var readAt *string
	if notification.ReadAt != nil {
		formatted := formatTime(*notification.ReadAt)
		readAt = &formatted
	}
	return Item{
		ID:         notification.ID,
		Type:       notification.Type,
		Level:      notification.Level,
		Title:      logger.RedactText(notification.Title),
		Content:    logger.RedactText(notification.Content),
		SourceType: notification.SourceType,
		SourceID:   notification.SourceID,
		Route:      notification.Route,
		IsRead:     notification.IsRead,
		CreatedAt:  formatTime(notification.CreatedAt),
		ReadAt:     readAt,
	}
}

// formatTime 输出 RFC3339 时间，空值保持空字符串，便于前端直接展示或格式化。
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
