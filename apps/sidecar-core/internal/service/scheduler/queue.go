package scheduler

import (
	"context"
	"sync"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/model"
)

// ExecutionQueue 是进程内执行队列，负责按 run_key 和数据范围做入队去重。
type ExecutionQueue struct {
	mu         sync.Mutex
	ready      *sync.Cond
	keys       map[string]struct{}
	activeKeys map[string]struct{}
	items      []model.SchedulerRun
}

// NewExecutionQueue 创建本地执行队列。
func NewExecutionQueue() *ExecutionQueue {
	queue := &ExecutionQueue{
		keys:       make(map[string]struct{}),
		activeKeys: make(map[string]struct{}),
	}
	queue.ready = sync.NewCond(&queue.mu)
	return queue
}

// Enqueue 将执行记录放入队列，重复 run_key 或相同目标日期的数据范围会返回 added=false。
func (queue *ExecutionQueue) Enqueue(ctx context.Context, run model.SchedulerRun) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if _, ok := queue.keys[run.RunKey]; ok {
		return false, nil
	}
	activeKey := queueActiveKey(run)
	if _, ok := queue.activeKeys[activeKey]; ok {
		return false, nil
	}
	queue.keys[run.RunKey] = struct{}{}
	queue.activeKeys[activeKey] = struct{}{}
	queue.items = append(queue.items, run)
	queue.ready.Signal()
	return true, nil
}

// Dequeue 等待并返回下一个待执行记录，ctx 取消时返回取消错误。
func (queue *ExecutionQueue) Dequeue(ctx context.Context) (model.SchedulerRun, error) {
	if err := ctx.Err(); err != nil {
		return model.SchedulerRun{}, err
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()

	cancelled := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			queue.mu.Lock()
			queue.ready.Broadcast()
			queue.mu.Unlock()
		case <-cancelled:
		}
	}()
	defer close(cancelled)

	for len(queue.items) == 0 {
		queue.ready.Wait()
		if err := ctx.Err(); err != nil {
			return model.SchedulerRun{}, err
		}
	}
	index := queue.highestPriorityIndex()
	run := queue.items[index]
	queue.items = append(queue.items[:index], queue.items[index+1:]...)
	return run, nil
}

// Complete 释放执行范围锁，让同一数据范围和目标日期后续可以重新入队。
func (queue *ExecutionQueue) Complete(run model.SchedulerRun) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	delete(queue.activeKeys, queueActiveKey(run))
}

// Snapshot 返回当前队列快照，仅供管理接口和测试读取，不暴露内部切片。
func (queue *ExecutionQueue) Snapshot() []model.SchedulerRun {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	items := make([]model.SchedulerRun, len(queue.items))
	copy(items, queue.items)
	return items
}

// highestPriorityIndex 返回当前等待队列中优先级最高的元素位置；同优先级保持 FIFO。
func (queue *ExecutionQueue) highestPriorityIndex() int {
	if len(queue.items) == 0 {
		panic("scheduler queue is empty")
	}
	index := 0
	for current := 1; current < len(queue.items); current++ {
		if queue.items[current].Priority > queue.items[index].Priority {
			index = current
		}
	}
	return index
}

// queueActiveKey 返回队列中的执行范围锁 key，同一天相同数据范围只允许一个待执行记录。
func queueActiveKey(run model.SchedulerRun) string {
	if run.DataType != "" || run.ScopeKey != "" || run.Period != "" || run.TargetDate != "" {
		return run.DataType + "|" + run.ScopeKey + "|" + run.Period + "|" + run.TargetDate
	}
	return "run_key|" + run.RunKey
}
