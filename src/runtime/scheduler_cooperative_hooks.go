//go:build (scheduler.tasks || scheduler.asyncify) && !rtos.preempt

package runtime

import "internal/task"

func schedulerInit() {}

func schedulerEnqueue(t *task.Task) { runqueue.Push(t) }
func schedulerYield(t *task.Task)   { runqueue.Push(t) }
