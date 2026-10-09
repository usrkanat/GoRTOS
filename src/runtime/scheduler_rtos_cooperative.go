//go:build rtos && scheduler.tasks && !rtos.preempt

package runtime

import "internal/task"

func schedulerPopReady() *task.Task { return runqueue.PopPriority() }
