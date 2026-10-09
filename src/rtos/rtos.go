//go:build tinygo && rtos && scheduler.tasks

// Package rtos provides tasks with fixed priorities using the TinyGo runtime.
package rtos

import (
	"runtime"
	"time"
	_ "unsafe"
)

type Priority uint8

const (
	Lowest  Priority = 0
	Normal  Priority = 3
	Highest Priority = 7
)

// Start queues fn at priority. It does not yield the current task.
func Start(priority Priority, fn func()) {
	checkPriority(priority)
	if fn == nil {
		panic("rtos: nil task")
	}
	runtimeStart(uint8(priority), fn)
}

func CurrentPriority() Priority {
	return Priority(runtimePriority())
}

// SetPriority changes the current task's priority and yields.
func SetPriority(priority Priority) {
	checkPriority(priority)
	runtimeSetPriority(uint8(priority))
}

func Yield() {
	runtime.Gosched()
}

func Sleep(duration time.Duration) {
	if duration <= 0 {
		Yield()
		return
	}
	time.Sleep(duration)
}

// SleepUntil sleeps until deadline, or yields if it has passed.
func SleepUntil(deadline time.Time) {
	Sleep(time.Until(deadline))
}

// SleepForever blocks the current task without stopping other tasks.
func SleepForever() {
	select {}
}

func checkPriority(priority Priority) {
	if priority > Highest {
		panic("rtos: priority must be between 0 and 7")
	}
}

func runtimeStart(priority uint8, fn func())
func runtimePriority() uint8
func runtimeSetPriority(priority uint8)
