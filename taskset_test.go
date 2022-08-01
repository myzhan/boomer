package boomer

import (
	"reflect"
	"testing"
)

func TestWeighingTaskSetWithSingleTask(t *testing.T) {
	ts := NewWeighingTaskSet()

	taskAIsRun := false
	taskA := &Task{
		Name:   "A",
		Weight: 1,
		Fn: func() {
			taskAIsRun = true
		},
	}
	ts.AddTask(taskA)

	if ts.GetTask(0).Name != "A" {
		t.Error("Expecting A, but got ", ts.GetTask(0).Name)
	}
	if ts.GetTask(1) != nil {
		t.Error("Out of bound, should return nil")
	}
	if ts.GetTask(-1) != nil {
		t.Error("Out of bound, should return nil")
	}

	ts.Run()

	if !taskAIsRun {
		t.Error("Task A should be run")
	}
}

func TestWeighingTaskSetWithTwoTasks(t *testing.T) {
	ts := NewWeighingTaskSet()
	taskA := &Task{
		Name:   "A",
		Weight: 1,
	}
	taskB := &Task{
		Name:   "B",
		Weight: 2,
	}
	ts.AddTask(taskA)
	ts.AddTask(taskB)

	if ts.GetTask(0).Name != "A" {
		t.Error("Expecting A, but got ", ts.GetTask(0).Name)
	}
	if ts.GetTask(1).Name != "B" {
		t.Error("Expecting B, but got ", ts.GetTask(1).Name)
	}
}

func TestWeighingTaskSetGetTaskWithThreeTasks(t *testing.T) {
	ts := NewWeighingTaskSet()
	taskA := &Task{
		Name:   "A",
		Weight: 1,
	}
	taskB := &Task{
		Name:   "B",
		Weight: 2,
	}
	taskC := &Task{
		Name:   "C",
		Weight: 3,
	}
	ts.AddTask(taskA)
	ts.AddTask(taskB)
	ts.AddTask(taskC)

	if ts.GetTask(0).Name != "A" {
		t.Error("Expecting A, but got ", ts.GetTask(0).Name)
	}
	if ts.GetTask(1).Name != "B" {
		t.Error("Expecting B, but got ", ts.GetTask(1).Name)
	}
	if ts.GetTask(2).Name != "B" {
		t.Error("Expecting B, but got ", ts.GetTask(2).Name)
	}
	if ts.GetTask(3).Name != "C" {
		t.Error("Expecting C, but got ", ts.GetTask(3).Name)
	}
	if ts.GetTask(4).Name != "C" {
		t.Error("Expecting C, but got ", ts.GetTask(4).Name)
	}
	if ts.GetTask(5).Name != "C" {
		t.Error("Expecting C, but got ", ts.GetTask(5).Name)
	}
}

func TestSmoothRoundRobinTaskSetRun(t *testing.T) {
	ts := NewSmoothRoundRobinTaskSet()
	results := []string{}
	taskA := &Task{
		Name:   "A",
		Weight: 5,
		Fn: func() {
			results = append(results, "A")
		},
	}
	taskB := &Task{
		Name:   "B",
		Weight: 1,
		Fn: func() {
			results = append(results, "B")
		},
	}
	taskC := &Task{
		Name:   "C",
		Weight: 1,
		Fn: func() {
			results = append(results, "C")
		},
	}
	ts.AddTask(taskA)
	ts.AddTask(taskB)
	ts.AddTask(taskC)

	for i := 0; i < 7; i++ {
		ts.Run()
	}

	expected := []string{"A", "A", "B", "A", "C", "A", "A"}
	if !reflect.DeepEqual(results, expected) {
		t.Errorf("Expecting %v, but got %v\n", expected, results)
	}

}
