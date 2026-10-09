//go:build rtos.hosttest

package kernel

import (
	"rtoshost/arm"
	"rtoshost/task"
	"testing"
)

func reset(t *task.Task) {
	runqueue = task.Queue{}
	task.UseCurrent(t)
	task.SchedulePreempted = schedulePreemptedTask
}

func TestBlockedWakeIsQueuedOnce(t *testing.T) {
	current := &task.Task{RunState: task.RunStateRunning}
	reset(current)
	task.Pause()
	if task.Switches() != 1 || current.RunState != task.RunStatePaused {
		t.Fatal("task did not block")
	}
	schedulerEnqueue(current)
	schedulerEnqueue(current)
	if !current.Ready || current.WakePending {
		t.Fatal("blocked wake has invalid state")
	}
	if schedulerPopReady() != current || schedulerPopReady() != nil {
		t.Fatal("wake queued the task more than once")
	}
}

func TestWakeAcrossPreemption(t *testing.T) {
	for _, before := range []bool{false, true} {
		t.Run(map[bool]string{false: "after preemption", true: "before preemption"}[before], func(t *testing.T) {
			current := &task.Task{RunState: task.RunStateRunning}
			reset(current)
			if before {
				schedulerEnqueue(current)
			}
			if !task.Preempt() {
				t.Fatal("running task was not preempted")
			}
			if !before {
				schedulerEnqueue(current)
			}
			if !current.WakePending || !current.Ready {
				t.Fatal("wake was lost while task was preempted")
			}
			if schedulerPopReady() != current || schedulerPopReady() != nil {
				t.Fatal("preempted task was not queued exactly once")
			}
			task.Pause()
			if task.Switches() != 0 || current.WakePending || current.RunState != task.RunStateRunning {
				t.Fatal("task blocked despite pending wake")
			}
			if task.PreemptionMask() != 0 {
				t.Fatal("Pause did not restore preemption")
			}
		})
	}
}

func TestYieldPreservesPendingWake(t *testing.T) {
	current := &task.Task{RunState: task.RunStateRunning}
	reset(current)
	schedulerEnqueue(current)
	schedulerYield(current)
	task.Pause()
	if task.Switches() != 1 || !current.WakePending {
		t.Fatal("Yield skipped switching or consumed a wake")
	}
	if schedulerPopReady() != current {
		t.Fatal("yielded task is missing")
	}
	task.Pause()
	if task.Switches() != 1 || current.WakePending {
		t.Fatal("next blocking call did not consume the pending wake")
	}
}

func TestPreemptionRejectsBlockedOrQueuedTask(t *testing.T) {
	current := &task.Task{RunState: task.RunStatePaused}
	reset(current)
	if task.Preempt() {
		t.Fatal("blocked task was preempted")
	}
	current.RunState = task.RunStateRunning
	schedulerYield(current)
	if task.Preempt() {
		t.Fatal("already queued task was preempted")
	}
}

func TestPriorityAfterPreemption(t *testing.T) {
	a := &task.Task{RunState: task.RunStateRunning}
	b := &task.Task{}
	a.Priority, b.Priority = 3, 7
	reset(a)
	schedulerEnqueue(b)
	task.Preempt()
	if schedulerPopReady() != b || schedulerPopReady() != a {
		t.Fatal("preemption did not preserve priority ordering")
	}
}

func TestNestedRuntimeCriticalSection(t *testing.T) {
	reset(nil)
	var lock task.PMutex
	lock.Lock()
	lock.Lock()
	lock.Unlock()
	if task.PreemptionMask() != 0xf0 {
		t.Fatal("nested unlock enabled preemption")
	}
	lock.Unlock()
	if task.PreemptionMask() != 0 {
		t.Fatal("outer unlock did not restore preemption")
	}
}

func TestTimerConfiguration(t *testing.T) {
	arm.SCB.AIRCR.Set(0x700)
	arm.SCB.SHPR2.Set(0xff000000)
	schedulerInit()
	if arm.TimerReload != 71999 || arm.SCB.SHPR2.Get()>>24 != 0 {
		t.Fatal("invalid SysTick reload or SVC priority")
	}
	if arm.SCB.SHPR3.Get()>>16 != 0xe0f0 || arm.SCB.AIRCR.Get()&0x700 != 0 {
		t.Fatal("invalid interrupt priorities")
	}
	rtosTick()
	if arm.SCB.ICSR.Get() != 1<<28 {
		t.Fatal("SysTick did not request PendSV")
	}
}
