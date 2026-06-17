package analysis

import (
	"errors"
	"strings"
	"testing"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/task"
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
		code    ErrorCode
	}{
		{name: "missing ai config", request: CreateRequest{Symbol: "US:AAPL", AnalysisType: AnalysisTechnical, PromptTemplateID: 2}, code: ErrorMissingAIConfig},
		{name: "missing prompt template", request: CreateRequest{Symbol: "US:AAPL", AnalysisType: AnalysisTechnical, AIConfigID: 1}, code: ErrorMissingPromptTemplate},
		{name: "unsupported analysis type", request: CreateRequest{Symbol: "US:AAPL", AnalysisType: AnalysisType("portfolio"), AIConfigID: 1, PromptTemplateID: 2}, code: ErrorUnsupportedAnalysisType},
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
	assertAnalysisErrorCode(t, err, ErrorTaskAlreadyTerminal)
}

// assertAnalysisErrorCode 校验分析任务错误码稳定。
func assertAnalysisErrorCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}

	var analysisError *Error
	if !errors.As(err, &analysisError) {
		t.Fatalf("expected analysis Error, got %T", err)
	}
	if analysisError.Code != code {
		t.Fatalf("expected error code %q, got %q", code, analysisError.Code)
	}
}
