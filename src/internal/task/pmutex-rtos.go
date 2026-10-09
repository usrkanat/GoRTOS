//go:build rtos.preempt && rtos && scheduler.tasks && stm32f103

package task

import "runtime/interrupt"

type PMutex struct {
	mask  uintptr
	depth uint32
}

func (m *PMutex) Lock() {
	mask := interrupt.Disable()
	if m.depth == 0 {
		m.mask = DisablePreemption()
	}
	m.depth++
	interrupt.Restore(mask)
}

func (m *PMutex) Unlock() {
	mask := interrupt.Disable()
	if m.depth == 0 {
		runtimeFatal("runtime: unlock of unlocked PMutex")
	}
	m.depth--
	if m.depth == 0 {
		RestorePreemption(m.mask)
	}
	interrupt.Restore(mask)
}
