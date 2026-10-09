//go:build bluepill && rtos

package main

import (
	"machine"
	"rtos"
	"time"
)

func main() {
	machine.LED.Configure(machine.PinConfig{Mode: machine.PinOutput})
	rtos.Start(rtos.Normal, blink)
	rtos.Start(rtos.Highest, heartbeat)
	rtos.SleepForever()
}

func blink() {
	on := false
	for {
		on = !on
		machine.LED.Set(on)
		rtos.Sleep(500 * time.Millisecond)
	}
}

func heartbeat() {
	next := time.Now()
	for {
		println("rtos heartbeat")
		next = next.Add(time.Second)
		rtos.SleepUntil(next)
	}
}
