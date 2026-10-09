//go:build !rtos

package task

type priorityState struct{}

func (t *Task) inheritPriority() {}
