# RTOS tasks for Blue Pill

This package uses a modified TinyGo runtime with fixed-priority scheduling.
The `bluepill-rtos` target enables timer preemption on STM32F103C8.

Use the Blue Pill target with STM32F103C8 and an ST-Link programmer.
Build the compiler from this checkout first using the repository's build
instructions. An installed upstream TinyGo does not contain this package or
the scheduler changes.

From the repository root:

```sh
TINYGOROOT="$PWD" ./build/tinygo build -target=bluepill-rtos -o /tmp/rtos.hex ./src/examples/rtos
TINYGOROOT="$PWD" ./build/tinygo flash -target=bluepill-rtos ./src/examples/rtos
```

An application can import the package without a separate module dependency:

```go
package main

import (
    "rtos"
    "time"
)

func main() {
    rtos.Start(rtos.Highest, func() {
        for {
            println("sample")
            rtos.Sleep(100 * time.Millisecond)
        }
    })
    rtos.SleepForever()
}
```

Build your application with the same compiler and `-target=bluepill-rtos`.
You cannot import this package with the standard Go compiler or an unmodified
TinyGo installation.

With a built compiler, run the cooperative scheduler integration tests on the host:

```sh
TINYGOROOT="$PWD" ./build/tinygo test -tags=rtos -scheduler=tasks ./tests/rtos
```

Priorities range from 0 to 7. Higher values run first. The main task starts at
priority 0. `rtos.Start` assigns the priority before adding the new task to the
ready queue and does not explicitly yield. Ordinary `go` statements inherit
the parent's priority. Equal priorities run in FIFO order when tasks yield,
block, or exhaust their time slice.

SysTick requests a context switch every 1 ms. PendSV saves registers and
returns to the scheduler, which selects the highest-priority ready task.
SVC performs voluntary switches and starts or resumes tasks. The context
contains r0-r12, LR, PC, xPSR, EXC_RETURN, and BASEPRI. This implementation
supports Cortex-M3 on STM32F103, which has no FPU.

Runtime critical sections defer PendSV with BASEPRI. Timekeeping and peripheral
interrupts remain enabled. The allocator, collector, task queues, and blocking
operations use this protection. Wakes that arrive before a task pauses remain
pending across timer preemption.

`rtos.SetPriority` changes the current task's priority and yields.
`rtos.Sleep` yields for zero or negative durations.
`rtos.SleepUntil` accepts an absolute deadline to avoid accumulating drift
in periodic tasks. `rtos.SleepForever` blocks the caller and keeps other tasks
running. Returning from `main` still ends the program.

The scheduler makes every overdue sleeping task ready before selecting the
highest priority. Channels and timers continue to use the existing runtime.
The ready queue uses a linear priority search and remains compatible with
the existing garbage collector's queue scan.

CPU loops at equal priorities share the processor without calling `Yield` or
`Sleep`. A continuously ready higher-priority task still starves lower
priorities. `Yield` does not override priority ordering. If main must create
several CPU tasks before they run, first call `rtos.SetPriority(rtos.Highest)`.
There is no priority inheritance or hard deadline guarantee. Interrupts,
critical sections, and GC can delay a switch. Do not block from interrupt
handlers or with interrupts disabled.

The target reserves SysTick, SVC, PendSV, the priority grouping, and BASEPRI.
Do not replace their handlers or change their priorities. TIM4 remains
reserved for TinyGo timekeeping. Each goroutine uses a fixed 2048-byte stack;
increase `-stack-size` if needed and account for the board's 20 KiB of SRAM.

The example toggles the onboard LED every 500 ms and prints a heartbeat over
the target's UART once per second.

To exercise preemption on hardware, flash the CPU-loop example:

```sh
TINYGOROOT="$PWD" ./build/tinygo flash -target=bluepill-rtos ./src/examples/rtos-preempt
```

Two tasks increment separate counters without yielding. A high-priority
monitor sleeps for 500 ms, checks that both progressed, toggles the LED, and
prints `preemption OK` over UART. A stopped heartbeat is a failed check.

The assembly context tests need Clang with an ARM backend, Unicorn, and
pyelftools. They execute the production handlers with simulated exception
entry and return and a stub for the Go scheduler callback:

```sh
python3 tools/test-rtos-context.py
```

These tests verify register and stack preservation, BASEPRI nesting, and
rejected switches. They do not emulate NVIC scheduling or run a TinyGo
firmware. Full firmware and hardware validation are still required.

Run the kernel logic tests with host stubs for hardware instructions:

```sh
python3 tools/test-rtos-kernel.py
```

They exercise the production queue, wake bookkeeping, pause logic, and
critical-section code, including wakes before and after timer preemption.
