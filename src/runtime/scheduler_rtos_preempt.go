//go:build rtos.preempt && rtos && scheduler.tasks && stm32f103

package runtime

import (
	"device/arm"
	"internal/task"
	"machine"
	"runtime/interrupt"
)

func schedulerInit() {
	arm.SCB.CCR.SetBits(1 << 9)
	arm.SCB.AIRCR.Set(0x05fa0000 | (arm.SCB.AIRCR.Get() & 0x0000f8ff))
	arm.SCB.SHPR2.ClearBits(0xff << 24)
	arm.SCB.SHPR3.Set((arm.SCB.SHPR3.Get() & 0x0000ffff) | 0xe0<<24 | 0xf0<<16)
	if err := arm.SetupSystemTimer(machine.CPUFrequency()/1000 - 1); err != nil {
		panic("rtos: cannot configure SysTick")
	}
}

//export SysTick_Handler
func rtosTick() {
	arm.SCB.ICSR.Set(1 << 28)
}

func schedulerEnqueue(t *task.Task) {
	mask := interrupt.Disable()
	if t.RunState == task.RunStateRunning {
		t.WakePending = true
	} else if !t.Ready {
		t.Ready = true
		runqueue.Push(t)
	}
	interrupt.Restore(mask)
}

func schedulerYield(t *task.Task) {
	schedulePreemptedTask(t)
}

func schedulePreemptedTask(t *task.Task) {
	mask := interrupt.Disable()
	if !t.Ready {
		t.Ready = true
		runqueue.Push(t)
	}
	interrupt.Restore(mask)
}

func schedulerPopReady() *task.Task {
	mask := interrupt.Disable()
	t := runqueue.PopPriority()
	if t != nil {
		t.Ready = false
		t.RunState = task.RunStateRunning
	}
	interrupt.Restore(mask)
	return t
}
