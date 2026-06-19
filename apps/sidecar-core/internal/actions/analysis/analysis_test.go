package analysis

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/actions/httpx"
	analysisservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/analysis"
)

type blockingExecutor struct {
	started chan context.Context
}

// Execute 暴露执行上下文，方便测试取消注册是否在 goroutine 启动前已经完成。
func (executor *blockingExecutor) Execute(ctx context.Context, _ string, _ analysisservice.ValidatedCreateRequest, _ string) error {
	executor.started <- ctx
	<-ctx.Done()
	return ctx.Err()
}

// TestStartExecutorRegistersCancellationBeforeReturning 验证创建任务返回前已经可以取消后台执行。
func TestStartExecutorRegistersCancellationBeforeReturning(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)

	executor := &blockingExecutor{started: make(chan context.Context, 1)}
	runningTasks := NewRunningTasks()

	startExecutor(Config{
		Executor:     executor,
		RunningTasks: runningTasks,
	}, "task-race", analysisservice.ValidatedCreateRequest{}, "sk-runtime-secret", httpx.RequestContext{
		RequestID: "req-race",
		TraceID:   "trace-race",
	})
	cancelledBeforeExecutorStart := runningTasks.Cancel("task-race")

	var ctx context.Context
	select {
	case ctx = <-executor.started:
	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}
	if !cancelledBeforeExecutorStart {
		runningTasks.Cancel("task-race")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("executor context was not cancelled")
	}
	if !cancelledBeforeExecutorStart {
		t.Fatal("running task was not cancellable immediately after startExecutor returned")
	}
}
