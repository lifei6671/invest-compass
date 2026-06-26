package analysis

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/task"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

// TestValidateCreateRequestAcceptsSupportedAnalysis 验证合法分析请求会标准化 symbol 并保留本次持仓输入。
func TestValidateCreateRequestAcceptsSupportedAnalysis(t *testing.T) {
	request := CreateRequest{
		Symbol:           "cn:sh:600519",
		AnalysisType:     AnalysisStockFull,
		AIConfigID:       1,
		PromptTemplateID: 2,
		UserPosition: &UserPosition{
			CostPrice: 1680,
			Shares:    100,
			RiskLevel: "medium",
		},
	}

	validated, err := ValidateCreateRequest(request)
	if err != nil {
		t.Fatalf("ValidateCreateRequest returned error: %v", err)
	}

	if validated.Symbol.String() != "CN:SH:600519" {
		t.Fatalf("expected normalized symbol, got %q", validated.Symbol.String())
	}
	if validated.UserPosition == nil || validated.UserPosition.Shares != 100 {
		t.Fatalf("expected one-time user position in validated request: %+v", validated.UserPosition)
	}
}

// TestValidateCreateRequestRejectsMissingConfig 验证缺少模型配置和模板配置会返回明确错误。
func TestValidateCreateRequestRejectsMissingConfig(t *testing.T) {
	tests := []struct {
		name    string
		request CreateRequest
		code    xerr.Code
	}{
		{name: "missing ai config", request: CreateRequest{Symbol: "US:AAPL", AnalysisType: AnalysisTechnical, PromptTemplateID: 2}, code: xerr.AnalysisMissingAIConfig},
		{name: "missing prompt template", request: CreateRequest{Symbol: "US:AAPL", AnalysisType: AnalysisTechnical, AIConfigID: 1}, code: xerr.AnalysisMissingPromptTemplate},
		{name: "unsupported analysis type", request: CreateRequest{Symbol: "US:AAPL", AnalysisType: AnalysisType("portfolio"), AIConfigID: 1, PromptTemplateID: 2}, code: xerr.AnalysisUnsupportedType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateCreateRequest(tt.request)
			assertAnalysisErrorCode(t, err, tt.code)
		})
	}
}

// TestInputSnapshotForLogRedactsUserPosition 验证一次性持仓输入不会进入普通日志。
func TestInputSnapshotForLogRedactsUserPosition(t *testing.T) {
	request := ValidatedCreateRequest{
		AnalysisType:     AnalysisStockFull,
		AIConfigID:       1,
		PromptTemplateID: 2,
		UserPosition: &UserPosition{
			CostPrice: 1680,
			Shares:    100,
			RiskLevel: "medium",
		},
	}

	snapshot := request.InputSnapshotForLog()

	if strings.Contains(snapshot, "1680") || strings.Contains(snapshot, "100") || strings.Contains(snapshot, "medium") {
		t.Fatalf("log snapshot leaked user position: %s", snapshot)
	}
	if !strings.Contains(snapshot, "has_user_position") {
		t.Fatalf("log snapshot should keep non-sensitive position presence: %s", snapshot)
	}
}

// TestInputSnapshotForReportKeepsOneTimePosition 验证报告输入快照保留本次分析的一次性持仓输入。
func TestInputSnapshotForReportKeepsOneTimePosition(t *testing.T) {
	validated, err := ValidateCreateRequest(CreateRequest{
		Symbol:           "US:AAPL",
		AnalysisType:     AnalysisStockFull,
		AIConfigID:       7,
		PromptTemplateID: 9,
		UserPosition: &UserPosition{
			CostPrice: 123.45,
			Shares:    10,
			RiskLevel: "medium",
		},
	})
	if err != nil {
		t.Fatalf("ValidateCreateRequest returned error: %v", err)
	}

	snapshot := validated.InputSnapshotForReport()

	for _, expected := range []string{"US:AAPL", "stock_full", "123.45", "10", "medium"} {
		if !strings.Contains(snapshot, expected) {
			t.Fatalf("report snapshot missing %q: %s", expected, snapshot)
		}
	}
	for _, forbidden := range []string{"resolved_api_key", "raw_api_key", "api_key"} {
		if strings.Contains(snapshot, forbidden) {
			t.Fatalf("report snapshot leaked credential field %q: %s", forbidden, snapshot)
		}
	}
}

// TestCreateTaskBuildsPendingTaskAndSafeCreatedEvent 验证分析创建请求会落成待执行任务和安全创建事件。
func TestCreateTaskBuildsPendingTaskAndSafeCreatedEvent(t *testing.T) {
	validated, err := ValidateCreateRequest(CreateRequest{
		Symbol:           "cn:sh:600519",
		AnalysisType:     AnalysisStockFull,
		AIConfigID:       1,
		PromptTemplateID: 2,
		RetryOfTaskID:    "analysis-old",
		UserPosition: &UserPosition{
			CostPrice: 1680,
			Shares:    100,
			RiskLevel: "medium",
		},
	})
	if err != nil {
		t.Fatalf("ValidateCreateRequest returned error: %v", err)
	}

	now := time.Date(2026, 6, 17, 10, 30, 0, 0, time.UTC)
	createdTask, event := CreateTask(validated, "task_abc123", now)

	if createdTask.ID != "task_abc123" || createdTask.Type != task.TypeAnalysis || createdTask.Status != task.StatusPending {
		t.Fatalf("unexpected created task: %+v", createdTask)
	}
	if createdTask.CreatedAt != now || createdTask.UpdatedAt != now {
		t.Fatalf("created task should use supplied timestamp: %+v", createdTask)
	}
	if !strings.Contains(createdTask.Title, "CN:SH:600519") {
		t.Fatalf("created task title should include normalized symbol: %s", createdTask.Title)
	}
	if event.TaskID != "task_abc123" || event.Type != task.EventCreated {
		t.Fatalf("unexpected created event: %+v", event)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
		t.Fatalf("created event payload should be JSON: %v", err)
	}
	if payload["symbol"] != "CN:SH:600519" || payload["analysis_type"] != string(AnalysisStockFull) {
		t.Fatalf("created event payload missing safe summary: %s", event.Payload)
	}
	if payload["has_user_position"] != true {
		t.Fatalf("created event payload should keep position presence only: %s", event.Payload)
	}
	if payload["retry_of_task_id"] != "analysis-old" {
		t.Fatalf("created event payload should keep retry source task id: %s", event.Payload)
	}
	for _, secret := range []string{"1680", "100", "medium"} {
		if strings.Contains(event.Payload, secret) {
			t.Fatalf("created event payload leaked user position %q: %s", secret, event.Payload)
		}
	}
}

// TestCancelTaskTransitionsRunningTask 验证取消运行中任务会进入 CANCELLED。
func TestCancelTaskTransitionsRunningTask(t *testing.T) {
	cancelled, event, err := CancelTask(task.Task{ID: "task-1", Status: task.StatusRunning})
	if err != nil {
		t.Fatalf("CancelTask returned error: %v", err)
	}

	if cancelled.Status != task.StatusCancelled {
		t.Fatalf("expected CANCELLED, got %q", cancelled.Status)
	}
	if event.Type != task.EventCancelled || event.TaskID != "task-1" {
		t.Fatalf("unexpected cancel event: %+v", event)
	}
}

// TestCancelTaskRejectsTerminalTask 验证终态任务不能重复取消。
func TestCancelTaskRejectsTerminalTask(t *testing.T) {
	_, _, err := CancelTask(task.Task{ID: "task-1", Status: task.StatusSuccess})
	assertAnalysisErrorCode(t, err, xerr.AnalysisTaskAlreadyTerminal)
}

// assertAnalysisErrorCode 校验分析任务错误码稳定。
func assertAnalysisErrorCode(t *testing.T, err error, code xerr.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var analysisError *xerr.Error
	if !errors.As(err, &analysisError) {
		t.Fatalf("expected analysis Error, got %T", err)
	}
	if analysisError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, analysisError.Code)
	}
}
