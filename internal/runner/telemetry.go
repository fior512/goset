package runner

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/isolation"
	"goset/internal/report"
	"goset/internal/telemetry"
)

func startTelemetry(selected *generic.Selection, cfg *cli.Config) (func(time.Duration, error, *isolation.SteerResult, *syscall.Rusage), error) {
	// HK pin
	pinHousekeeper := func() error {
		var mask generic.CPUSet
		mask.SetBit(selected.HouseKeeper)
		return cpu.SetAffinity(0, mask)
	}

	// record
	sampler := &telemetry.Sampler{
		Cpus:     selected.Task,
		Interval: time.Duration(cfg.SamplingMS) * time.Millisecond,
		Sources: []telemetry.Source{
			&telemetry.IRQSource{},
			&telemetry.ThrottleSource{},
			&telemetry.FreqSource{},
		},
		Exit: make(chan struct{}),
		Done: make(chan struct{}),
	}
	if err := sampler.Start(pinHousekeeper); err != nil {
		return nil, err
	}

	// lazy print
	stop := func(wall time.Duration, runErr error, steer *isolation.SteerResult, rusage *syscall.Rusage) {
		fmt.Fprintln(os.Stdout, "\n\n--- GOSET ---")
		rep := report.Report{Steer: steer, Rusage: rusage, Wall: wall, Counters: sampler.Stop(), ExitCode: exitCode(runErr)}
		report.Render(os.Stdout,
			report.SelectionTable(selected),
			report.TelemetryTable(rep),
			report.GlobalTable(rep))
	}
	return stop, nil
}
