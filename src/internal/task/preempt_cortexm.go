//go:build rtos.preempt && rtos && scheduler.tasks && stm32f103

package task

import _ "unsafe"

type preemptState struct {
	Ready       bool
	WakePending bool
}

//export tinygo_rtos_disablePreemption
func DisablePreemption() uintptr

//export tinygo_rtos_restorePreemption
func RestorePreemption(mask uintptr)

func taskStackSize(size uintptr) uintptr { return (size + 7) &^ 7 }

//export tinygo_rtos_preempt
func preempt() bool {
	t := Current()
	if t == nil || t.RunState != RunStateRunning || t.Ready {
		return false
	}
	if *t.state.canaryPtr != stackCanary {
		runtimeFatal("goroutine stack overflow")
	}
	schedulePreemptedTask(t)
	return true
}

//go:linkname schedulePreemptedTask runtime.schedulePreemptedTask
func schedulePreemptedTask(t *Task)
