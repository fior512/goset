package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/isolation"
)

func Run(cfg *cli.Config) (err error) {
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// Topology
	topo, err := cpu.GetTopology()
	if err != nil {
		return err
	}

	// Select threads
	selected, err := cpu.SelectCPUs(topo, cfg.NThreads, cfg.Include, cfg.Exclude, cfg.Numa, cfg.Cgroup && cfg.Fence)
	if err != nil {
		return err
	}

	// Isolation (steer/cgroup)
	group, steering, release, err := startIsolation(cfg, topo, selected)
	if err != nil {
		return err
	}
	defer func() { // destroy cgroup + counter-steer
		failed := release()
		if failed == nil {
			return
		}
		fmt.Fprintf(os.Stderr, "%steardown: %v\n", generic.LogPrefix, failed)
		if err == nil {
			err = errors.New("isolation teardown incomplete")
		}
	}()

	// Telemetry
	stop, err := startTelemetry(selected, cfg, steering)
	if err != nil {
		return err
	}

	// Pin + run task
	rusage, wall, runErr := isolation.ApplyPin(ctx, cfg.Task, selected.Task, group)
	stop(wall, runErr, steering, rusage) // Lazy telemetry report
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
