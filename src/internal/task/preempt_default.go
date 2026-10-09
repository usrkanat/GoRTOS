//go:build !rtos.preempt

package task

type preemptState struct{}

func DisablePreemption() uintptr { return 0 }
func RestorePreemption(uintptr)  {}

func taskStackSize(size uintptr) uintptr { return size }
