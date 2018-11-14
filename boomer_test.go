package boomer

import (
	"testing"
)

func TestRunTasksForTest(t *testing.T) {
	count := 0
	taskA := &Task{
		Name: "increaseCount",
		Fn: func() {
			count++
		},
	}
	runTasks = "increaseCount,foobar"
	runTasksForTest(taskA)

	if count != 1 {
		t.Error("count is", count, "expected: 1")
	}
}
