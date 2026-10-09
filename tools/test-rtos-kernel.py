#!/usr/bin/env python3
"""Run production scheduler logic with host stubs for Cortex-M instructions."""

import os
import pathlib
import subprocess
import tempfile


ROOT = pathlib.Path(__file__).resolve().parents[1]


def main():
    with tempfile.TemporaryDirectory(prefix="rtos-kernel-") as directory:
        work = pathlib.Path(directory)

        def write(name, source):
            target = work / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(source)

        def source(name):
            text = (ROOT / name).read_text()
            for old, new in [
                ('"internal/task"', '"rtoshost/task"'),
                ('"runtime/interrupt"', '"rtoshost/interrupt"'),
                ('"device/arm"', '"rtoshost/arm"'),
                ('"machine"', '"rtoshost/machine"'),
            ]:
                text = text.replace(old, new)
            return text

        write("go.mod", "module rtoshost\n\ngo 1.25\n")
        write("interrupt/interrupt.go", """package interrupt
type State uintptr
var state State
func Disable() State { old := state; state = 1; return old }
func Restore(old State) { state = old }
func In() bool { return false }
""")
        write("arm/arm.go", """package arm
type Register uint32
func (r *Register) Get() uint32 { return uint32(*r) }
func (r *Register) Set(v uint32) { *r = Register(v) }
func (r *Register) SetBits(v uint32) { *r |= Register(v) }
func (r *Register) ClearBits(v uint32) { *r &^= Register(v) }
var SCB struct { CCR, AIRCR, SHPR2, SHPR3, ICSR Register }
var TimerReload uint32
func SetupSystemTimer(cycles uint32) error { TimerReload = cycles; return nil }
""")
        write("machine/machine.go", "package machine\nfunc CPUFrequency() uint32 { return 72000000 }\n")
        queue = source("src/internal/task/queue.go")
        queue = queue.split("// Use runtime.lockAtomics")[0]
        write("task/queue.go", queue.replace('import "rtoshost/interrupt"\n', ''))
        for name in ["priority_rtos.go", "priority_rtos_test.go", "pmutex-rtos.go"]:
            write("task/" + name, source("src/internal/task/" + name))
        context = source("src/internal/task/task_stack_rtos_cortexm.go").replace('import "C"\n', '')
        for declaration, body in [
            ("func switchToTask(sp *uintptr)", "func switchToTask(sp *uintptr) {}"),
            ("func switchToScheduler()", "func switchToScheduler() { switches++ }"),
            ("func SystemStack() uintptr", "func SystemStack() uintptr { return 0 }"),
            ("func GoroutineStack() uintptr", "func GoroutineStack() uintptr { return 0 }"),
        ]:
            context = context.replace(declaration + "\n", body + "\n")
        write("task/context.go", context)
        preempt = source("src/internal/task/preempt_cortexm.go")
        preempt = preempt.replace("func DisablePreemption() uintptr\n", "")
        preempt = preempt.replace("func RestorePreemption(mask uintptr)\n", "")
        preempt = preempt.replace("//go:linkname schedulePreemptedTask runtime.schedulePreemptedTask\n", "")
        preempt = preempt.replace("func schedulePreemptedTask(t *Task)\n", "")
        write("task/preempt.go", preempt)
        write("task/host.go", """package task
import "rtoshost/interrupt"
type Task struct { priorityState; preemptState; Next *Task; RunState uint8; state state; gcData gcData }
type state struct { sp uintptr; canaryPtr *uintptr }
type gcData struct{}
func (*gcData) swap() {}
var startTask [0]uint8
const stackCanary uintptr = 0x55
const ( RunStatePaused = iota; RunStateRunning )
var switches int
var preemptionMask uintptr
var SchedulePreempted func(*Task)
func schedulePreemptedTask(t *Task) { SchedulePreempted(t) }
func runtimeFatal(message string) { panic(message) }
func lockAtomics() interrupt.State { return interrupt.Disable() }
func unlockAtomics(mask interrupt.State) { interrupt.Restore(mask) }
func DisablePreemption() uintptr { old := preemptionMask; preemptionMask = 0xf0; return old }
func RestorePreemption(mask uintptr) { preemptionMask = mask }
func UseCurrent(t *Task) {
    currentTask = t
    switches = 0
    preemptionMask = 0
    if t != nil && t.state.canaryPtr == nil { canary := stackCanary; t.state.canaryPtr = &canary }
}
func Switches() int { return switches }
func PreemptionMask() uintptr { return preemptionMask }
func Preempt() bool { return preempt() }
""")
        scheduler = source("src/runtime/scheduler_rtos_preempt.go").replace("package runtime", "package kernel", 1)
        write("kernel/scheduler.go", scheduler)
        write("kernel/host.go", "package kernel\nimport \"rtoshost/task\"\nvar runqueue task.Queue\n")
        write("kernel/kernel_test.go", source("tests/rtos_preempt/kernel_test.go"))
        environment = os.environ.copy()
        environment["GOCACHE"] = str(work / "cache")
        subprocess.run([
            "go", "test", "-v", "-tags=rtos,rtos.preempt,scheduler.tasks,stm32f103,rtos.hosttest", "./...",
        ], cwd=work, env=environment, check=True)


if __name__ == "__main__":
    main()
