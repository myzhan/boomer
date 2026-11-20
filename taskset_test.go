package boomer

import (
	"fmt"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test taskset", func() {

	It("test weighing taskset with single task", func() {
		ts := NewWeighingTaskSet()
		Expect(ts.GetTask(0)).To(BeNil())

		taskAIsRun := false
		taskA := &Task{
			Name:   "A",
			Weight: 1,
			Fn: func() {
				taskAIsRun = true
			},
		}
		ts.AddTask(taskA)

		Expect(ts.GetTask(0).Name).To(Equal("A"))
		Expect(ts.GetTask(1)).To(BeNil())
		Expect(ts.GetTask(-1)).To(BeNil())

		ts.Run()
		Expect(taskAIsRun).To(BeTrue())
	})

	It("test weighing taskset with two tasks", func() {
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

		Expect(ts.GetTask(0).Name).To(Equal("A"))
		Expect(ts.GetTask(1).Name).To(Equal("B"))
	})

	It("test weighing taskset get task with three tasks", func() {
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

		Expect(ts.GetTask(0).Name).To(Equal("A"))
		Expect(ts.GetTask(1).Name).To(Equal("B"))
		Expect(ts.GetTask(2).Name).To(Equal("B"))
		Expect(ts.GetTask(3).Name).To(Equal("C"))
		Expect(ts.GetTask(3).Name).To(Equal("C"))
		Expect(ts.GetTask(3).Name).To(Equal("C"))
	})

	It("test smooth round robin taskset run", func() {
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
		Expect(results).To(BeEquivalentTo(expected))
	})

})

func TestSmoothRoundRobinTaskSetConcurrentAccess(t *testing.T) {
	ts := NewSmoothRoundRobinTaskSet()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				ts.AddTask(&Task{
					Name:   fmt.Sprintf("task-%d-%d", worker, j),
					Weight: 1,
					Fn:     func() {},
				})
				if task := ts.GetTask(); task != nil {
					task.Fn()
				}
				ts.SetWeight(j)
				_ = ts.GetWeight()
			}
		}(i)
	}
	wg.Wait()

	if ts.GetTask() == nil {
		t.Fatal("expected a task after adding tasks")
	}
}

func TestWeighingTaskSetConcurrentAccess(t *testing.T) {
	ts := NewWeighingTaskSet()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				ts.AddTask(&Task{
					Name:   fmt.Sprintf("task-%d-%d", worker, j),
					Weight: 1,
					Fn:     func() {},
				})
				if task := ts.GetTask(0); task != nil {
					task.Fn()
				}
				ts.SetWeight(j)
				_ = ts.GetWeight()
			}
		}(i)
	}
	wg.Wait()

	if ts.GetTask(0) == nil {
		t.Fatal("expected a task after adding tasks")
	}
}

// Run used to panic with "invalid argument to Intn" when the task set was empty.
func TestWeighingTaskSetRunWithoutTasks(t *testing.T) {
	ts := NewWeighingTaskSet()
	ts.Run()
}

func TestTaskSetAddNilTask(t *testing.T) {
	smooth := NewSmoothRoundRobinTaskSet()
	smooth.AddTask(nil)

	weighing := NewWeighingTaskSet()
	weighing.AddTask(nil)
}
