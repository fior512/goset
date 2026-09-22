package runner

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/isolation"
	"goset/internal/report"
	"goset/internal/telemetry"
)

func startTelemetry(selected *generic.Selection, cfg *cli.Config) (func(error, *isolation.SteerResult, *exec.Cmd), error) {
	pinHousekeeper := func() error {
		var mask generic.CPUSet
		mask.SetBit(selected.HouseKeeper)
		return cpu.SetAffinity(0, mask)
	}

	sampler := &telemetry.Sampler{
		Cpus:     selected.Task,
		Interval: time.Duration(cfg.SamplingMS) * time.Millisecond,
		Sources: []telemetry.Source{
			&telemetry.IRQSource{},
			&telemetry.ThrottleSource{},
			&telemetry.FreqSource{},
		},
	}

	//HouseKeeper
	if err := sampler.Start(pinHousekeeper); err != nil {
		return nil, err
	}
	start := time.Now()

	// lazy print
	stop := func(runErr error, steer *isolation.SteerResult, task *exec.Cmd) {
		fmt.Fprintln(os.Stderr, "\n\n----------------- GOSET -----------------")
		var rusage *syscall.Rusage
		if task.ProcessState != nil {
			rusage, _ = task.ProcessState.SysUsage().(*syscall.Rusage)
		}
		rep := report.Report{Steer: steer, Rusage: rusage, Wall: time.Since(start), Counters: sampler.Stop(), ExitCode: exitCode(runErr)}
		report.Render(os.Stderr,
			report.SelectionTable(selected),
			report.TelemetryTable(rep),
			report.GlobalTable(rep))
	}
	return stop, nil
}
