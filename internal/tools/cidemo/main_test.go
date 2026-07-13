package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
)

type fakeExecutor struct {
	calls     [][]string
	failOn    string
	failed    bool
	cleanupOK bool
}

func (e *fakeExecutor) Run(_ context.Context, args []string, _, _ io.Writer) error {
	e.calls = append(e.calls, slices.Clone(args))
	if args[0] == e.failOn && !e.failed {
		e.failed = true
		return errors.New("injected failure")
	}
	if e.failed && args[0] == "down" {
		e.cleanupOK = true
	}
	return nil
}

func TestRunDemoExecutesCompleteWorkflow(t *testing.T) {
	t.Parallel()
	executor := &fakeExecutor{}
	var stdout bytes.Buffer
	if err := runDemo(context.Background(), &stdout, io.Discard, executor); err != nil {
		t.Fatalf("runDemo() error = %v", err)
	}
	steps := demoSteps()
	if len(executor.calls) != len(steps) {
		t.Fatalf("calls = %d, want %d", len(executor.calls), len(steps))
	}
	for index, step := range steps {
		if !slices.Equal(executor.calls[index], step.args) {
			t.Fatalf("call %d = %v, want %v", index, executor.calls[index], step.args)
		}
	}
	if !strings.Contains(stdout.String(), "Clean container demo passed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunDemoCleansUpAfterStartedStepFails(t *testing.T) {
	t.Parallel()
	executor := &fakeExecutor{failOn: "verify"}
	err := runDemo(context.Background(), io.Discard, io.Discard, executor)
	if err == nil || !strings.Contains(err.Error(), "Verify deterministic invariants") {
		t.Fatalf("runDemo() error = %v", err)
	}
	if !executor.cleanupOK {
		t.Fatalf("calls = %v, want cleanup down after failure", executor.calls)
	}
	if got := executor.calls[len(executor.calls)-1]; !slices.Equal(got, []string{"down"}) {
		t.Fatalf("last call = %v, want cleanup down", got)
	}
}

func TestRunDemoDoesNotCleanUpBeforeStartup(t *testing.T) {
	t.Parallel()
	executor := &fakeExecutor{failOn: "doctor"}
	err := runDemo(context.Background(), io.Discard, io.Discard, executor)
	if err == nil {
		t.Fatal("runDemo() error = nil, want injected failure")
	}
	for _, call := range executor.calls {
		if call[0] == "down" {
			t.Fatalf("calls = %v, did not want cleanup before startup", executor.calls)
		}
	}
}
