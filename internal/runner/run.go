package runner

import (
	"errors"
	"os/exec"
	"time"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/isolation"
)

func Run(cfg *cli.Config) error {
	topo, err := cpu.GetTopology()
	if err != nil {
		return err
	}
	n := max(cfg.Cgroup, 1)

	// Selection
	selection, err := cpu.SelectCPUs(topo, n, cfg.Include, cfg.Exclude, cfg.NumaNode)
	if err != nil {
		return err
	}

	// Cgroup
	// TODO: define if cgroup with single thread is worth
	var group *isolation.Cgroup
	if n > 1 { // TODO: ducktape check
		var err error
		group, err = isolation.InitCgroup(cfg.Task[0], selection.Benchmark, -1) // TODO: handle NumaNode
		if err != nil {
			return err
		}
	}

	// Telemetry
	stop, err := startTelemetry(selection, cfg)
	if err != nil {
		return err
	}

	started := time.Now()
	runErr := isolation.ApplyPin(cfg.Task, selection.Benchmark, group) //TODO: extract Task.start()
	stop(time.Since(started), runErr) //lazy-print returned by startTelemetry

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
	return -1 //TODO: find better undefined
}
