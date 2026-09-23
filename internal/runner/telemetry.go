package runner

import (
	"fmt"
	"os"
	"time"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/isolation"
	"goset/internal/report"
	"goset/internal/telemetry"
)

func startTelemetry(selected *generic.Selection, cfg *cli.Config) (func(*isolation.TaskResult, *isolation.SteerResult), error) {
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

	// HouseKeeper
	if err := sampler.Start(pinHousekeeper); err != nil {
		return nil, err
	}

	// lazy print
	stop := func(res *isolation.TaskResult, steer *isolation.SteerResult) {
		fmt.Fprintln(os.Stderr, "\n\n----------------- GOSET -----------------")
		rep := report.Report{
			Steer:    steer,
			Rusage:   res.Rusage,
			Wall:     res.Wall,
			Counters: sampler.Stop(),
			ExitCode: exitCode(res.Err),
		}
		report.Render(os.Stderr,
			report.SelectionTable(selected),
			report.TelemetryTable(rep),
			report.GlobalTable(rep))
	}
	return stop, nil
}
