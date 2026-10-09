//go:build tinygo && rtos && scheduler.tasks && !rtos.preempt

package rtos_test

import (
	"reflect"
	"rtos"
	"testing"
	"time"
)

func TestReadyOrder(t *testing.T) {
	done := make(chan struct{}, 4)
	var order []int
	for i, priority := range []rtos.Priority{1, 7, 3, 7} {
		rtos.Start(priority, func() {
			order = append(order, i)
			done <- struct{}{}
		})
	}
	if len(order) != 0 {
		t.Fatal("Start yielded")
	}
	for range 4 {
		<-done
	}
	if !reflect.DeepEqual(order, []int{1, 3, 2, 0}) {
		t.Fatalf("task order = %v", order)
	}
	if rtos.CurrentPriority() != rtos.Lowest {
		t.Fatal("Start changed the parent's priority")
	}
}

func TestEqualPriorityYield(t *testing.T) {
	done := make(chan struct{}, 2)
	var order []int
	for i := range 2 {
		rtos.Start(rtos.Normal, func() {
			order = append(order, i)
			rtos.Sleep(0)
			order = append(order, i)
			done <- struct{}{}
		})
	}
	for range 2 {
		<-done
	}
	if !reflect.DeepEqual(order, []int{0, 1, 0, 1}) {
		t.Fatalf("task order = %v", order)
	}
}

func TestInheritedPriority(t *testing.T) {
	done := make(chan rtos.Priority, 3)
	rtos.Start(rtos.Highest, func() {
		done <- rtos.CurrentPriority()
		go func() {
			done <- rtos.CurrentPriority()
		}()
		rtos.SetPriority(rtos.Normal)
		done <- rtos.CurrentPriority()
	})
	for _, want := range []rtos.Priority{rtos.Highest, rtos.Highest, rtos.Normal} {
		if got := <-done; got != want {
			t.Fatalf("priority = %d, want %d", got, want)
		}
	}
}

func TestOverdueSleepers(t *testing.T) {
	deadline := time.Now().Add(30 * time.Millisecond)
	done := make(chan rtos.Priority, 2)
	started := make(chan struct{}, 2)
	for _, priority := range []rtos.Priority{rtos.Normal, rtos.Highest} {
		rtos.Start(priority, func() {
			started <- struct{}{}
			when := deadline
			if priority == rtos.Normal {
				when = when.Add(-10 * time.Millisecond)
			}
			rtos.SleepUntil(when)
			done <- rtos.CurrentPriority()
		})
	}
	for range 2 {
		<-started
	}
	for time.Now().Before(deadline.Add(10 * time.Millisecond)) {
	}
	rtos.Yield()
	for _, want := range []rtos.Priority{rtos.Highest, rtos.Normal} {
		if got := <-done; got != want {
			t.Fatalf("wake priority = %d, want %d", got, want)
		}
	}
}
