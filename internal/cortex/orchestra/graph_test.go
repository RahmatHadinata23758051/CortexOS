package orchestra

import (
	"errors"
	"fmt"
	"testing"
)

func graphTask(id TaskID, dependencies ...TaskID) Task {
	task := validTask()
	task.ID = id
	task.Dependencies = dependencies
	return task
}

func TestValidateGraphRejectsMissingDuplicateAndCyclicDependencies(t *testing.T) {
	if err := ValidateGraph([]Task{graphTask("a", "missing")}); !errors.Is(err, ErrMissingDependency) {
		t.Fatalf("missing dependency error = %v", err)
	}
	if err := ValidateGraph([]Task{graphTask("a"), graphTask("a")}); !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("duplicate task error = %v", err)
	}
	if err := ValidateGraph([]Task{graphTask("a", "b"), graphTask("b", "a")}); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestReadyTasksAreBoundedAndDeterministic(t *testing.T) {
	tasks := []Task{
		graphTask("b"),
		graphTask("c", "a"),
		graphTask("a"),
		graphTask("d", "a"),
	}
	tasks[2].Status = TaskSuccess
	ready, err := ReadyTasks(tasks)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 3 || ready[0].ID != "b" || ready[1].ID != "c" || ready[2].ID != "d" {
		t.Fatalf("ready = %#v", ready)
	}
}

func TestReadyTasksWaitsForSuccessfulDependencies(t *testing.T) {
	tasks := []Task{graphTask("a"), graphTask("b", "a")}
	ready, err := ReadyTasks(tasks)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 || ready[0].ID != "a" {
		t.Fatalf("ready = %#v", ready)
	}
	tasks[0].Status = TaskSuccess
	ready, err = ReadyTasks(tasks)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 || ready[0].ID != "b" {
		t.Fatalf("ready after dependency success = %#v", ready)
	}
}

func TestValidateGraphRejectsOversizedPlans(t *testing.T) {
	tasks := make([]Task, MaxPlanTasks+1)
	for i := range tasks {
		tasks[i] = graphTask(TaskID(fmt.Sprintf("task-%03d", i)))
	}
	if err := ValidateGraph(tasks); !errors.Is(err, ErrPlanTooLarge) {
		t.Fatalf("oversized graph error = %v", err)
	}
}
