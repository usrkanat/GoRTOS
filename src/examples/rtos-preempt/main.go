//go:build bluepill && rtos.preempt

package main

import (
	"machine"
	"rtos"
	"sync/atomic"
	"time"
)

var counters [2]atomic.Uint32

func main() {
	machine.LED.Configure(machine.PinConfig{Mode: machine.PinOutput})
	rtos.SetPriority(rtos.Highest)
	for i := range counters {
		rtos.Start(rtos.Normal, func() {
			for {
				counters[i].Add(1)
			}
		})
	}
	rtos.Start(rtos.Highest, monitor)
	rtos.SleepForever()
}

func monitor() {
	on := false
	for {
		before := [2]uint32{counters[0].Load(), counters[1].Load()}
		rtos.Sleep(500 * time.Millisecond)
		a := counters[0].Load() - before[0]
		b := counters[1].Load() - before[1]
		if a == 0 || b == 0 {
			panic("rtos: a CPU task did not run")
		}
		println("preemption OK", a, b)
		on = !on
		machine.LED.Set(on)
	}
}
