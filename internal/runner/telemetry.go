package runner

import (
	"fmt"
	"os"
	"time"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/report"
	"goset/internal/telemetry"
)

/*
	Policy: No IPI
*/

func startTelemetry(selection *cpu.SelectionResult, cfg *cli.Config) (func(time.Duration, error), error) {
	// Translate CPUSet
	var benchCPUs []int
	for c := range selection.Benchmark.All() {
		benchCPUs = append(benchCPUs, c)
	}

	// HK pin
	pinHousekeeper := func() error {
		var mask cpu.CPUSet
		mask.SetBit(selection.HouseKeeper)
		return cpu.SetAffinity(0, mask)
	}

	// record
	sampler := &telemetry.Sampler{
		Cpus:     benchCPUs,
		Interval: time.Duration(cfg.SamplingMS) * time.Millisecond,
		Sources: []telemetry.Source{
			&telemetry.IRQSource{},
			&telemetry.ThrottleSource{},
		},
		Exit: make(chan struct{}),
		Done: make(chan struct{}),
	}
	if err := sampler.Start(pinHousekeeper); err != nil {
		return nil, err
	}

	// lazy print
	stop := func(wall time.Duration, runErr error) {
		fmt.Fprintln(os.Stdout, "\n\n--- GOSET ---\n")
		rep := report.Report{Wall: wall, Counters: sampler.Stop(), ExitCode: exitCode(runErr)}
		report.Render(os.Stdout,
			report.SelectionTable(selection),
			report.TelemetryTable(rep),
			report.GlobalTable(rep))
	}
	return stop, nil
}
