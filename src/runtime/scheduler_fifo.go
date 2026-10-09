//go:build (scheduler.tasks || scheduler.asyncify) && !rtos

package runtime

import "internal/task"

func schedulerPopTask() *task.Task {
	return runqueue.Pop()
}
