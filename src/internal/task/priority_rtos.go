//go:build rtos && scheduler.tasks

package task

type priorityState struct {
	Priority uint8
}

func (t *Task) inheritPriority() {
	if parent := Current(); parent != nil {
		t.Priority = parent.Priority
	}
}

// PopPriority removes the first task with the highest priority.
func (q *Queue) PopPriority() *Task {
	mask := lockAtomics()
	defer unlockAtomics(mask)
	best := q.head
	if best == nil {
		return nil
	}
	var previous, bestPrevious *Task
	for t := q.head; t != nil; t = t.Next {
		if t.Priority > best.Priority {
			best, bestPrevious = t, previous
		}
		previous = t
	}
	if bestPrevious == nil {
		q.head = best.Next
	} else {
		bestPrevious.Next = best.Next
	}
	if q.tail == best {
		q.tail = bestPrevious
	}
	best.Next = nil
	return best
}
