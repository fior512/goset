package telemetry

import "goset/internal/generic"

type Target struct {
	BenchCPUs   generic.CPUSet
	Housekeeper int
}

type Counter struct {
	Source string
	CPU    int
	Name   string
	Value  float64
}

type Source interface {
	Baseline(target Target) error // starting point
	Poll() error                  // sample
	Stop() error                  // ending point
	Summary() []Counter           // report
}
