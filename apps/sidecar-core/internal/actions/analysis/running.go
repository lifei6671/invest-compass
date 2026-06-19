package analysis

import (
	"context"
	"sync"
)

// RunningTasks 记录当前进程内正在执行的分析任务取消函数。
type RunningTasks struct {
	mutex sync.Mutex
	items map[string]*runningTask
}

type runningTask struct {
	cancel context.CancelFunc
}

// NewRunningTasks 创建分析任务运行期注册表，只保存内存态取消函数。
func NewRunningTasks() *RunningTasks {
	return &RunningTasks{items: make(map[string]*runningTask)}
}

// Start 注册运行中任务并返回可取消上下文和清理函数。
func (tasks *RunningTasks) Start(taskID string) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	item := &runningTask{cancel: cancel}

	tasks.mutex.Lock()
	if previous, ok := tasks.items[taskID]; ok {
		previous.cancel()
	}
	tasks.items[taskID] = item
	tasks.mutex.Unlock()

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			tasks.mutex.Lock()
			if tasks.items[taskID] == item {
				delete(tasks.items, taskID)
			}
			tasks.mutex.Unlock()
			cancel()
		})
	}
	return ctx, cleanup
}

// Cancel 取消指定运行中任务；任务不在当前进程执行时返回 false。
func (tasks *RunningTasks) Cancel(taskID string) bool {
	tasks.mutex.Lock()
	item, ok := tasks.items[taskID]
	tasks.mutex.Unlock()
	if !ok {
		return false
	}
	item.cancel()
	return true
}
