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

	// Selection
	selection, err := cpu.SelectCPUs(topo, cfg.NThreads, cfg.Include, cfg.Exclude, cfg.NumaNode)
	if err != nil {
		return err
	}

	// Cgroup
	var group *isolation.Cgroup
	if cfg.Cgroup {
		var err error
		group, err = isolation.InitCgroup("goset-"+cfg.Task[0], selection.Benchmark, -1) // rename // TODO: handle NumaNode
		if err != nil {
			return err
		}
	}

	// IRQ steering
	var steer *isolation.SteerResult
	if cfg.Steering {
		steer, err = isolation.SteerIRQs(topo, selection.Benchmark)
		if err != nil {
			return err
		}
		defer isolation.RestoreIRQs(steer)
	}

	// Telemetry
	stop, err := startTelemetry(selection, cfg)
	if err != nil {
		return err
	}

	// Run task
	started := time.Now()
	rusage, runErr := isolation.ApplyPin(cfg.Task, selection.Benchmark, group, cfg.NThreads > 0) // TODO: extract Task.start()
	stop(time.Since(started), runErr, steer, rusage) // lazy-print returned by startTelemetry

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
