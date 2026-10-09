//go:build rtos && scheduler.tasks

package runtime

import (
	"internal/task"
	"runtime/interrupt"
	_ "unsafe"
)

func schedulerPopTask() *task.Task {
	if sleepQueue != nil {
		now := ticks()
		for sleepQueue != nil && now-sleepQueueBaseTime >= timeUnit(sleepQueue.Data) {
			t := sleepQueue
			sleepQueueBaseTime += timeUnit(t.Data)
			sleepQueue = t.Next
			t.Next = nil
			scheduleTaskNoWake(t)
		}
	}
	return schedulerPopReady()
}

//go:linkname rtosStart rtos.runtimeStart
func rtosStart(priority uint8, fn func()) {
	if interrupt.In() {
		panic("rtos: cannot start a task from an interrupt")
	}
	preemption := task.DisablePreemption()
	defer task.RestorePreemption(preemption)
	parent := task.Current()
	previous := parent.Priority
	parent.Priority = priority
	go fn()
	parent.Priority = previous
}

//go:linkname rtosPriority rtos.runtimePriority
func rtosPriority() uint8 {
	if interrupt.In() {
		panic("rtos: no task priority in an interrupt")
	}
	return task.Current().Priority
}

//go:linkname rtosSetPriority rtos.runtimeSetPriority
func rtosSetPriority(priority uint8) {
	if interrupt.In() {
		panic("rtos: cannot change task priority from an interrupt")
	}
	preemption := task.DisablePreemption()
	defer task.RestorePreemption(preemption)
	task.Current().Priority = priority
	Gosched()
}
