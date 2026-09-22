package runner

import (
	"time"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/telemetry"
)

func startTelemetry(selected *generic.Selection, cfg *cli.Config) (*telemetry.Sampler, error) {
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
	if err := sampler.Start(pinHousekeeper); err != nil {
		return nil, err
	}
	return sampler, nil
}
