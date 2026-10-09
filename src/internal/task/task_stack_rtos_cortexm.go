//go:build rtos.preempt && rtos && scheduler.tasks && stm32f103

package task

import "C"
import (
	"runtime/interrupt"
	"unsafe"
)

// Software frame followed by the hardware exception frame, ARM DDI 0337E section 5.6.
// https://documentation-service.arm.com/static/6036810d5319e554d4ba108e
type calleeSavedRegs struct {
	basepri, excReturn                uintptr
	r4, r5, r6, r7, r8, r9, r10, r11  uintptr
	r0, r1, r2, r3, r12, lr, pc, xpsr uintptr
}

func (s *state) archInit(r *calleeSavedRegs, fn uintptr, args unsafe.Pointer) {
	s.sp = uintptr(unsafe.Pointer(r))
	r.excReturn = 0xfffffffd
	r.r4 = fn
	r.r5 = uintptr(args)
	r.pc = uintptr(unsafe.Pointer(&startTask)) &^ 1
	r.xpsr = 1 << 24
}

func (s *state) resume() { switchToTask(&s.sp) }
func (s *state) pause()  { switchToScheduler() }

//export tinygo_rtos_switchToTask
func switchToTask(sp *uintptr)

//export tinygo_rtos_switchToScheduler
func switchToScheduler()

//export SystemStack
func SystemStack() uintptr

//export GoroutineStack
func GoroutineStack() uintptr

var currentTask *Task

func Current() *Task { return currentTask }

func Pause() {
	if interrupt.In() {
		runtimeFatal("blocked inside interrupt")
	}
	preemption := DisablePreemption()
	mask := interrupt.Disable()
	if mask != 0 {
		runtimeFatal("blocked with interrupts disabled")
	}
	t := currentTask
	if *t.state.canaryPtr != stackCanary {
		runtimeFatal("goroutine stack overflow")
	}
	if t.RunState == RunStateRunning && t.WakePending && !t.Ready {
		t.WakePending = false
		interrupt.Restore(mask)
		RestorePreemption(preemption)
		return
	}
	if !t.Ready {
		t.RunState = RunStatePaused
	}
	interrupt.Restore(mask)
	t.state.pause()
	RestorePreemption(preemption)
}

func (t *Task) Resume() {
	currentTask = t
	t.gcData.swap()
	t.state.resume()
	t.gcData.swap()
	currentTask = nil
}
