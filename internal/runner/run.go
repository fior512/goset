package runner

import (
	"errors"
	"os/exec"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/isolation"
)

func Run(cfg *cli.Config) error {
	// Topology
	topo, err := cpu.GetTopology()
	if err != nil {
		return err
	}

	// Select threads
	selected, err := cpu.SelectCPUs(topo, cfg.NThreads, cfg.Include, cfg.Exclude, cfg.NumaNode)
	if err != nil {
		return err
	}

	// Isolation (steer/cgroup)
	group, steering, release, err := startIsolation(cfg, topo, selected)
	if err != nil {
		return err
	}
	defer release() // destroy cgroup + counter-steer

	// Telemetry
	stop, err := startTelemetry(selected, cfg, steering)
	if err != nil {
		return err
	}

	// Pin + run task
	rusage, wall, runErr := isolation.ApplyPin(cfg.Task, selected.Task, group)
	stop(wall, runErr, steering, rusage) // Lazy telemetry report
	return runErr
}

// exitCode task's exit code
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1 // TODO: find better undefined
}
