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

func Run(config *cli.Config) (err error) {
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	topo, err := cpu.GetTopology()
	if err != nil {
		return err
	}
	selection, err := cpu.Selection(topo, config)
	if err != nil {
		return err
	}
	cgroup, steering, release, err := Isolation(config, topo, selection)
	if err != nil {
		return err
	}
	defer release(&err)
	lazyReport, err := Telemetry(selection, config, steering)
	if err != nil {
		return err
	}
	rusage, wall, runErr := isolation.SpawnTask(ctx, config.Task, selection.Task, cgroup)
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
