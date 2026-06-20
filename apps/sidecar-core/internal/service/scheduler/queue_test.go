package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// TestExecutionQueueDeduplicatesActiveScope 验证同一 data_type、scope_key、period 同时只能有一个 active run。
func TestExecutionQueueDeduplicatesActiveScope(t *testing.T) {
	queue := NewExecutionQueue()
	first := model.SchedulerRun{
		RunKey:   "run-1",
		DataType: "quote",
		ScopeKey: "CN:SH:600519",
		Period:   "",
	}
	second := model.SchedulerRun{
		RunKey:   "run-2",
		DataType: "quote",
		ScopeKey: "CN:SH:600519",
		Period:   "",
	}

	added, err := queue.Enqueue(context.Background(), first)
	if err != nil {
		t.Fatalf("enqueue first run: %v", err)
	}
	if !added {
		t.Fatal("expected first run to be enqueued")
	}
	added, err = queue.Enqueue(context.Background(), second)
	if err != nil {
		t.Fatalf("enqueue duplicate scope run: %v", err)
	}
	if added {
		t.Fatal("expected duplicate active scope to be rejected")
	}
	if len(queue.Snapshot()) != 1 {
		t.Fatalf("expected one queued item, got %+v", queue.Snapshot())
	}
}

// TestExecutionQueueAllowsDifferentTargetDates 验证补偿不同交易日时允许同一 scope 排队。
func TestExecutionQueueAllowsDifferentTargetDates(t *testing.T) {
	queue := NewExecutionQueue()
	first := model.SchedulerRun{
		RunKey:     "run-2026-06-18",
		DataType:   "kline",
		ScopeKey:   "CN:SH:600519",
		Period:     "day",
		TargetDate: "2026-06-18",
	}
	second := model.SchedulerRun{
		RunKey:     "run-2026-06-19",
		DataType:   "kline",
		ScopeKey:   "CN:SH:600519",
		Period:     "day",
		TargetDate: "2026-06-19",
	}

	if added, err := queue.Enqueue(context.Background(), first); err != nil || !added {
		t.Fatalf("enqueue first run: added=%v err=%v", added, err)
	}
	if added, err := queue.Enqueue(context.Background(), second); err != nil || !added {
		t.Fatalf("enqueue second target date run: added=%v err=%v", added, err)
	}
	if len(queue.Snapshot()) != 2 {
		t.Fatalf("expected two target date runs, got %+v", queue.Snapshot())
	}
}

// TestExecutionQueueDequeuesHigherPriorityFirst 验证交互式手动任务会排在低优先级补偿任务前执行。
func TestExecutionQueueDequeuesHigherPriorityFirst(t *testing.T) {
	queue := NewExecutionQueue()
	backfill := model.SchedulerRun{
		RunKey:     "backfill-run",
		DataType:   "kline",
		ScopeKey:   "CN:SH:600519",
		Period:     "day",
		TargetDate: "2026-06-18",
		Priority:   60,
	}
	manual := model.SchedulerRun{
		RunKey:     "manual-run",
		DataType:   "quote",
		ScopeKey:   "CN:SH:000001",
		TargetDate: "2026-06-19",
		Priority:   100,
	}

	if added, err := queue.Enqueue(context.Background(), backfill); err != nil || !added {
		t.Fatalf("enqueue backfill run: added=%v err=%v", added, err)
	}
	if added, err := queue.Enqueue(context.Background(), manual); err != nil || !added {
		t.Fatalf("enqueue manual run: added=%v err=%v", added, err)
	}
	run, err := queue.Dequeue(context.Background())
	if err != nil {
		t.Fatalf("dequeue run: %v", err)
	}
	if run.RunKey != "manual-run" {
		t.Fatalf("expected highest priority run first, got %+v", run)
	}
}

// TestExecutionQueueDequeueReturnsWhenContextCancelled 验证空队列等待期间取消 context 会唤醒 worker。
func TestExecutionQueueDequeueReturnsWhenContextCancelled(t *testing.T) {
	queue := NewExecutionQueue()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := queue.Dequeue(ctx)
		result <- err
	}()

	cancel()
	select {
	case err := <-result:
		if err != context.Canceled {
			t.Fatalf("expected context canceled from empty queue dequeue, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("expected dequeue to return after context cancellation")
	}
}

// TestExecutionQueueCompleteReleasesScope 验证 worker 完成后同一 scope 可以再次入队。
func TestExecutionQueueCompleteReleasesScope(t *testing.T) {
	queue := NewExecutionQueue()
	first := model.SchedulerRun{
		RunKey:   "run-1",
		DataType: "quote",
		ScopeKey: "CN:SH:600519",
		Period:   "",
	}
	second := model.SchedulerRun{
		RunKey:   "run-2",
		DataType: "quote",
		ScopeKey: "CN:SH:600519",
		Period:   "",
	}
	if added, err := queue.Enqueue(context.Background(), first); err != nil || !added {
		t.Fatalf("enqueue first run: added=%v err=%v", added, err)
	}
	run, err := queue.Dequeue(context.Background())
	if err != nil {
		t.Fatalf("dequeue first run: %v", err)
	}
	queue.Complete(run)

	added, err := queue.Enqueue(context.Background(), second)
	if err != nil {
		t.Fatalf("enqueue second run after complete: %v", err)
	}
	if !added {
		t.Fatal("expected scope to be reusable after complete")
	}
}
