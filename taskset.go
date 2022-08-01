package boomer

import (
	"math/rand"
	"sync"
	"time"
)

// TaskSet is an experimental feature, the API is not stabilized.
// It needs to be more considered and tested.
type TaskSet interface {
	// Add a Task to the TaskSet.
	AddTask(task *Task)
	// Set the weight of the TaskSet.
	SetWeight(weight int)
	// Get the weight of the TaskSet.
	GetWeight() (weight int)
	// Run will pick up a Task from the TaskSet and run.
	Run()
}

// WeighingTaskSet is a implementation of the TaskSet interface.
// When a Task is added to WeighingTaskSet, it's weight is used
// to calculate the probability to be called.
type WeighingTaskSet struct {
	weight int
	offset int
	tasks  []*Task
	index  []int
	lock   sync.RWMutex
}

// NewWeighingTaskSet returns a new WeighingTaskSet.
func NewWeighingTaskSet() *WeighingTaskSet {
	return &WeighingTaskSet{
		weight: 0,
		offset: 0,
		tasks:  make([]*Task, 0),
		index:  make([]int, 0),
	}
}

// AddTask add a Task to the Weighing TaskSet.
// If the task's weight is <= 0, it will be ignored.
func (ts *WeighingTaskSet) AddTask(task *Task) {
	if task.Weight <= 0 {
		return
	}
	ts.lock.Lock()
	ts.tasks = append(ts.tasks, task)
	ts.offset = ts.offset + task.Weight
	ts.index = append(ts.index, ts.offset)
	ts.lock.Unlock()
}

func (ts *WeighingTaskSet) binarySearch(roll int) (task *Task) {
	left := 0
	right := len(ts.index) - 1
	for left <= right {
		mid := (left + right) / 2
		if left == mid {
			if roll < ts.index[mid] {
				return ts.tasks[mid]
			}
			return ts.tasks[mid+1]
		}
		if ts.index[mid-1] <= roll && roll < ts.index[mid] {
			return ts.tasks[mid]
		}
		if roll < ts.index[mid-1] {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}
	return nil
}

// GetTask returns a task in the task set.
func (ts *WeighingTaskSet) GetTask(roll int) (task *Task) {
	if roll < 0 || roll >= ts.offset {
		return nil
	}
	ts.lock.RLock()
	task = ts.binarySearch(roll)
	ts.lock.RUnlock()
	return task
}

// SetWeight sets the weight of the task set.
func (ts *WeighingTaskSet) SetWeight(weight int) {
	ts.weight = weight
}

// GetWeight returns the weight of the task set.
func (ts *WeighingTaskSet) GetWeight() (weight int) {
	return ts.weight
}

// Run will pick up a task in the task set randomly and run.
// It can is used as a Task.Fn.
func (ts *WeighingTaskSet) Run() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	roll := r.Intn(ts.offset)
	task := ts.GetTask(roll)
	task.Fn()
}

// roundRobinTask is used by SmoothRoundRobinTaskSet.
type roundRobinTask struct {
	Task

	currentWeight   int
	effectiveWeight int
}

func newRoundRobinTask(task *Task) *roundRobinTask {
	rrTask := &roundRobinTask{}
	rrTask.Fn = task.Fn
	rrTask.Weight = task.Weight
	rrTask.Name = task.Name
	rrTask.currentWeight = 0
	rrTask.effectiveWeight = task.Weight
	return rrTask
}

// Smooth weighted round-robin balancing algorithm as seen in Nginx.
// See aslo: https://github.com/linnik/roundrobin/blob/master/roundrobin/smooth_rr.py
type SmoothRoundRobinTaskSet struct {
	// The weight of taskset
	weight int

	rrTasks []*roundRobinTask
	lock    sync.RWMutex
}

// NewSmoothRoundRobinTaskSet returns a new SmoothRoundRobinTaskSet.
func NewSmoothRoundRobinTaskSet() *SmoothRoundRobinTaskSet {
	return &SmoothRoundRobinTaskSet{
		weight:  0,
		rrTasks: make([]*roundRobinTask, 0),
	}
}

// AddTask add a Task to the Smooth RoundRobin TaskSet.
// If the task's weight is <= 0, it will be ignored.
func (ts *SmoothRoundRobinTaskSet) AddTask(task *Task) {
	if task.Weight <= 0 {
		return
	}

	rrTask := newRoundRobinTask(task)
	ts.rrTasks = append(ts.rrTasks, rrTask)
}

func (ts *SmoothRoundRobinTaskSet) GetTask() (task *Task) {
	rrTasksLength := len(ts.rrTasks)

	if rrTasksLength == 0 {
		return nil
	}

	if rrTasksLength == 1 {
		return &ts.rrTasks[0].Task
	}

	totalWeight := 0
	var picked *roundRobinTask

	ts.lock.Lock()

	for _, rrTask := range ts.rrTasks {
		rrTask.currentWeight += rrTask.effectiveWeight
		totalWeight += rrTask.effectiveWeight
		if rrTask.effectiveWeight < rrTask.Weight {
			rrTask.effectiveWeight += 1
		}
		if picked == nil || picked.currentWeight < rrTask.currentWeight {
			picked = rrTask
		}
	}
	picked.currentWeight -= totalWeight

	ts.lock.Unlock()

	return &picked.Task
}

// SetWeight sets the weight of the task set.
func (ts *SmoothRoundRobinTaskSet) SetWeight(weight int) {
	ts.weight = weight
}

// GetWeight returns the weight of the task set.
func (ts *SmoothRoundRobinTaskSet) GetWeight() (weight int) {
	return ts.weight
}

// Run will pick up a task in the task set smoothly and run.
// It can is used as a Task.Fn.
func (ts *SmoothRoundRobinTaskSet) Run() {
	task := ts.GetTask()
	if task != nil {
		task.Fn()
	}
}
