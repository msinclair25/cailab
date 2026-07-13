// Command cidemo runs CloudAILab's deterministic clean-container workflow.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"time"
)

const (
	defaultBinary   = "/usr/local/bin/cailab"
	defaultStateDir = "/tmp/cloudailab-state"
)

type commandExecutor interface {
	Run(context.Context, []string, io.Writer, io.Writer) error
}

type processExecutor struct {
	binary   string
	stateDir string
}

func (e processExecutor) Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	command := exec.CommandContext(ctx, e.binary, args...)
	command.Env = []string{
		"CAILAB_HOME=" + e.stateDir,
		"HOME=/tmp",
		"PATH=/usr/local/bin",
	}
	command.Stdout = stdout
	command.Stderr = stderr
	command.WaitDelay = 2 * time.Second
	if err := command.Run(); err != nil {
		return fmt.Errorf("execute cailab %s: %w", args[0], err)
	}
	return nil
}

type demoStep struct {
	label string
	args  []string
}

func demoSteps() []demoStep {
	return []demoStep{
		{label: "Inspect version metadata", args: []string{"version"}},
		{label: "Check walking-skeleton prerequisites", args: []string{"doctor", "walking-skeleton"}},
		{label: "List the embedded catalog", args: []string{"scenario", "list"}},
		{label: "Show the walking-skeleton briefing", args: []string{"scenario", "show", "walking-skeleton"}},
		{label: "Start the deterministic scenario", args: []string{"up", "walking-skeleton"}},
		{label: "Inspect active status", args: []string{"status"}},
		{label: "Inspect the mission", args: []string{"mission"}},
		{label: "Explain the expected trust path", args: []string{"graph", "path", "google:alex", "aws:acquisition-data"}},
		{label: "Verify deterministic invariants", args: []string{"verify"}},
		{label: "Stop the scenario", args: []string{"down"}},
	}
}

func runDemo(ctx context.Context, stdout, stderr io.Writer, executor commandExecutor) error {
	active := false
	for _, step := range demoSteps() {
		fmt.Fprintf(stdout, "==> %s\n", step.label)
		if err := executor.Run(ctx, step.args, stdout, stderr); err != nil {
			if active {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				cleanupErr := executor.Run(cleanupCtx, []string{"down"}, io.Discard, stderr)
				cancel()
				return errors.Join(fmt.Errorf("run demo step %q: %w", step.label, err), cleanupErr)
			}
			return fmt.Errorf("run demo step %q: %w", step.label, err)
		}
		switch step.args[0] {
		case "up":
			active = true
		case "down":
			active = false
		}
	}
	fmt.Fprintln(stdout, "==> Clean container demo passed")
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	stateDir := os.Getenv("CAILAB_HOME")
	if stateDir == "" {
		stateDir = defaultStateDir
	}
	executor := processExecutor{binary: defaultBinary, stateDir: stateDir}
	if err := runDemo(ctx, os.Stdout, os.Stderr, executor); err != nil {
		fmt.Fprintf(os.Stderr, "clean container demo: %v\n", err)
		os.Exit(1)
	}
}
