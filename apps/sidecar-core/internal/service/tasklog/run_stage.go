package tasklog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/logger"
	"github.com/lifei6671/invest-compass/apps/sidecar-core/pkg/xerr"
)

const (
	// StageQuoteFetch 表示行情快照拉取阶段。
	StageQuoteFetch = "quote_fetch"
	// StageKlineFetch 表示 K 线数据拉取阶段。
	StageKlineFetch = "kline_fetch"
	// StageCalcMACD 表示技术指标计算阶段。
	StageCalcMACD = "calc_macd"
	// StagePromptBuild 表示 Prompt 构建阶段。
	StagePromptBuild = "prompt_build"
	// StageStreamStart 表示 AI 流式调用启动阶段。
	StageStreamStart = "stream_start"
	// StageStreamChunk 表示 AI 流式内容摘要阶段。
	StageStreamChunk = "stream_chunk"
	// StageStreamTimeout 表示 AI 流式响应超时阶段。
	StageStreamTimeout = "stream_timeout"
	// StageStreamFailed 表示 AI 流式调用失败阶段。
	StageStreamFailed = "stream_failed"
)

const defaultStageErrorCode = "stage_failed"

// RunStage 执行业务阶段并写入一条任务结构化日志。
//
// fn 的业务错误会原样返回；日志写入错误通过 errors.Join 附加，调用方仍能识别原始错误。
func RunStage(ctx context.Context, writer StageWriter, meta StageMeta, stage string, fn func(context.Context) error) error {
	if writer == nil {
		return fmt.Errorf("task log writer is required")
	}
	meta.TaskID = strings.TrimSpace(meta.TaskID)
	meta.Module = strings.TrimSpace(meta.Module)
	stage = strings.TrimSpace(stage)
	if meta.TaskID == "" {
		return fmt.Errorf("task_id is required")
	}
	if meta.Module == "" {
		return fmt.Errorf("module is required")
	}
	if stage == "" {
		return fmt.Errorf("stage is required")
	}
	if fn == nil {
		return fmt.Errorf("stage function is required")
	}

	startedAt := stageNow(meta)
	err := fn(ctx)
	durationMS := stageNow(meta).Sub(startedAt).Milliseconds()
	entry := buildStageEntry(meta, stage, err, durationMS)
	if writeErr := writer.WriteTaskLog(ctx, entry); writeErr != nil {
		if err != nil {
			return errors.Join(err, fmt.Errorf("write task log: %w", writeErr))
		}
		return fmt.Errorf("write task log: %w", writeErr)
	}
	return err
}

// buildStageEntry 根据阶段执行结果构造脱敏任务日志。
func buildStageEntry(meta StageMeta, stage string, stageErr error, durationMS int64) model.TaskLogEntry {
	level := LevelInfo
	message := stage + " completed"
	code := strings.TrimSpace(meta.Code)
	if stageErr != nil {
		level = LevelError
		message = logger.RedactError(stageErr)
		if code == "" {
			code = codeFromError(stageErr)
		}
	}
	payloadJSON := buildStagePayload(stage, stageErr, durationMS, code, meta.Retryable)
	return model.TaskLogEntry{
		TaskID:      logger.RedactText(meta.TaskID),
		RequestID:   logger.RedactText(meta.RequestID),
		TraceID:     logger.RedactText(meta.TraceID),
		Ts:          stageNow(meta).UTC(),
		Level:       level,
		Module:      logger.RedactText(strings.TrimSpace(meta.Module)),
		Stage:       logger.RedactText(stage),
		Message:     logger.RedactText(message),
		Code:        logger.RedactText(code),
		Provider:    logger.RedactText(meta.Provider),
		Model:       logger.RedactText(meta.Model),
		Symbol:      logger.RedactText(meta.Symbol),
		DurationMS:  durationMS,
		Retryable:   meta.Retryable,
		PayloadJSON: payloadJSON,
	}
}

// buildStagePayload 构造阶段日志中的脱敏结构化 payload。
func buildStagePayload(stage string, stageErr error, durationMS int64, code string, retryable bool) string {
	payload := map[string]any{
		"stage":       logger.RedactText(stage),
		"duration_ms": durationMS,
		"retryable":   retryable,
	}
	if stageErr == nil {
		payload["status"] = "success"
	} else {
		payload["status"] = "error"
		payload["error"] = logger.RedactError(stageErr)
		payload["code"] = logger.RedactText(code)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return logger.RedactText(string(encoded))
}

// codeFromError 从业务错误中提取稳定错误码。
func codeFromError(err error) string {
	var coded *xerr.Error
	if errors.As(err, &coded) && coded != nil && coded.Code != "" {
		return string(coded.Code)
	}
	return defaultStageErrorCode
}

// stageNow 返回阶段日志使用的当前时间。
func stageNow(meta StageMeta) time.Time {
	if meta.Now != nil {
		return meta.Now()
	}
	return time.Now().UTC()
}
