package telemetry

import "goset/internal/generic"

const (
	SourceIRQ      = "irq"
	SourceThrottle = "throttle"
	SourceFreq     = "freq"
	SourceSched    = "sched"
	SourceIRQSteer = "irq steer"
)

const (
	IRQSoft         = "soft"
	IRQHard         = "hard"
	ThrottleCount   = "count"
	FreqMin         = "min MHz"
	FreqMax         = "max MHz"
	FreqAvg         = "avg MHz"
	SchedRunDelay   = "run_delay"
	SchedMigrations = "nr_migrations"
	IRQSteerDrift   = "drift"
)

type Counter struct {
	Source string
	CPU    int
	Name   string
	Value  float64
}

func (counter Counter) Label() string {
	return counter.Source + " " + counter.Name
}

type Source interface {
	Baseline(selected generic.Selection) error // starting point
	Poll() error                  // sample
	Stop() error                  // ending point
	Summary() []Counter           // report
}
