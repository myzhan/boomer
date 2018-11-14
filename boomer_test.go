package boomer

import (
	"testing"
	"time"
)

func TestEnableRPSControl(t *testing.T) {
	maxRPS = int64(1000)
	requestIncreaseRate = "100/2s"
	enableRPSControl()

	if !rpsControlEnabled {
		t.Error("rpsControlEnabled is, expected: true", rpsControlEnabled)
	}
	if requestIncreaseStep != 100 {
		t.Error("requestIncreaseStep is", requestIncreaseStep, "expected:", 100)
	}
	if requestIncreaseInterval != 2*time.Second {
		t.Error("requestIncreaseInterval is", requestIncreaseInterval, "expected: 2s")
	}

	// parse again
	requestIncreaseRate = "200"
	enableRPSControl()
	if requestIncreaseStep != 200 {
		t.Error("requestIncreaseStep is", requestIncreaseStep, "expected:", 200)
	}
	if requestIncreaseInterval != time.Second {
		t.Error("requestIncreaseInterval is", requestIncreaseInterval, "expected: 1s")
	}
}

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
