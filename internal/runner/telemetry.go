package runner

import (
	"time"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/telemetry"
)

func startTelemetry(selection *cpu.SelectionResult, cfg *cli.Config) (*telemetry.Sampler, error) {
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
		Sources:  []telemetry.Source{telemetry.IRQ()},
		Exit:     make(chan struct{}),
		Done:     make(chan struct{}),
	}
	if err := sampler.Start(pinHousekeeper); err != nil {
		return nil, err
	}
	return sampler, nil
}
