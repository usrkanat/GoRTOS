//go:build rtos && scheduler.tasks

package task

import "testing"

func TestPopPriority(t *testing.T) {
	for _, tc := range []struct {
		name       string
		priorities []uint8
		order      []int
	}{
		{"empty", nil, nil},
		{"single", []uint8{3}, []int{0}},
		{"highest at head", []uint8{7, 1, 3}, []int{0, 2, 1}},
		{"highest at tail", []uint8{1, 3, 7}, []int{2, 1, 0}},
		{"equal priorities", []uint8{3, 3, 3}, []int{0, 1, 2}},
		{"mixed priorities", []uint8{1, 7, 3, 7, 0}, []int{1, 3, 2, 0, 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tasks := make([]Task, len(tc.priorities))
			var q Queue
			for i, priority := range tc.priorities {
				tasks[i].Priority = priority
				if i == 0 {
					q.head = &tasks[i]
				} else {
					tasks[i-1].Next = &tasks[i]
				}
				q.tail = &tasks[i]
			}
			for _, index := range tc.order {
				got := q.PopPriority()
				if got != &tasks[index] {
					t.Fatalf("popped %p, want task %d at %p", got, index, &tasks[index])
				}
				if got.Next != nil {
					t.Fatal("popped task still linked")
				}
				if q.tail != nil && q.tail.Next != nil {
					t.Fatal("tail has a successor")
				}
			}
			if q.PopPriority() != nil || q.head != nil || q.tail != nil {
				t.Fatal("queue is not empty after draining")
			}
		})
	}
}
