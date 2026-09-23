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

func startTelemetry(selected *generic.Selection, cfg *cli.Config, steering *isolation.Steering) (func(time.Duration, error, *isolation.Steering, *syscall.Rusage), error) {
	pinHousekeeper := func() error {
		var mask generic.CPUSet
		mask.SetBit(selected.HouseKeeper)
		return cpu.SetAffinity(0, mask)
	}

	sources := []telemetry.Source{
		&telemetry.IRQSource{},
		&telemetry.ThrottleSource{},
		&telemetry.FreqSource{},
		&telemetry.MigrationsSource{Root: generic.ProcRoot},
		&telemetry.RunqueueSource{Root: generic.ProcRoot},
	}
	if steering != nil {
		if expected := steering.ExpectedAffinities(); len(expected) > 0 {
			IRQDrift := &telemetry.IRQDriftSource{
				Root:     generic.ProcIRQ,
				Expected: expected,
				Drifted:  map[string]bool{},
			}

			sources = append(sources, IRQDrift)
		}
	}
	sampler := &telemetry.Sampler{
		Cpus:     selected.Task,
		Interval: time.Duration(cfg.SamplingMS) * time.Millisecond,
		Sources:  sources,
	}

	// HouseKeeper
	if err := sampler.Start(pinHousekeeper); err != nil {
		return nil, err
	}

	// lazy print
	stop := func(wall time.Duration, runErr error, steering *isolation.Steering, rusage *syscall.Rusage) {
		fmt.Fprintln(os.Stderr, "\n\n----------------- GOSET -----------------")
		rep := report.Report{
			Steer:    steering,
			Rusage:   rusage,
			Wall:     wall,
			Counters: sampler.Stop(),
			ExitCode: exitCode(runErr),
		}
		report.Render(os.Stderr,
			report.SelectionTable(selected),
			report.TelemetryTable(rep),
			report.GlobalTable(rep))
	}
	return stop, nil
}
