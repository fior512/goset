package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/isolation"
	"goset/internal/report"
)

func Run(cfg *cli.Config) error {
	topo, err := cpu.GetTopology()
	if err != nil {
		return err
	}

	// Selection
	selected, err := cpu.SelectCPUs(topo, cfg.NThreads, cfg.Include, cfg.Exclude, cfg.NumaNode)
	if err != nil {
		return err
	}

	// Cgroup
	var group *isolation.Cgroup
	if cfg.Cgroup {
		var err error
		cgroupName := generic.CgroupIdentifier + filepath.Base(cfg.Task[0])
		group, err = isolation.InitCgroup(cgroupName, selected.Task, cfg.NumaNode)
		if err != nil {
			return err
		}
	}

	// IRQ steering
	var steer *isolation.SteerResult
	if cfg.Steering {
		steer, err = isolation.SteerIRQs(topo, selected.Task, selected.HouseKeeper)
		if err != nil {
			group.Destroy()
			return err
		}
	}

	// Telemetry
	sampler, err := startTelemetry(selected, cfg)
	if err != nil {
		isolation.Teardown(group, steer)
		return err
	}

	// Run task
	// TODO: it is leaking
	started := time.Now()
	task, err := isolation.ApplyPin(cfg.Task, selected.Task, group)
	if err != nil {
		sampler.Stop()
		isolation.Teardown(group, steer)
		return err
	}
	rusage, runErr := isolation.WaitTask(task)

	rep := report.Report{
		Steer:    steer,
		Rusage:   rusage,
		Wall:     time.Since(started),
		Counters: sampler.Stop(),
		ExitCode: exitCode(runErr),
	}
	isolation.Teardown(group, steer)
	
	fmt.Fprintln(os.Stderr, "\n\n----------------- GOSET -----------------")
	report.Render(os.Stderr,
		report.SelectionTable(selected),
		report.TelemetryTable(rep),
		report.GlobalTable(rep))

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
