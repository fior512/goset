package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/isolation"
)

func Run(cfg *cli.Config) (err error) {
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	topo, err := cpu.GetTopology()
	if err != nil {
		return err
	}
	selected, err := cpu.Selection(topo, cfg)
	if err != nil {
		return err
	}
	group, steering, release, err := Isolation(cfg, topo, selected)
	if err != nil {
		return err
	}
	defer release(&err)
	lazyReport, err := Telemetry(selected, cfg, steering)
	if err != nil {
		return err
	}
	rusage, wall, runErr := isolation.SpawnTask(ctx, cfg.Task, selected.Task, group)
	lazyReport(wall, runErr, steering, rusage)
	return runErr
}

// ExitCode task's exit code, -1: task produced none
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		return exitErr.ExitCode()
	}
	return -1 // task never started, or failed outside the child itself
}
