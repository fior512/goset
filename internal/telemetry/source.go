package telemetry

import "goset/internal/generic"

type Counter struct {
	Source string
	CPU    int
	Name   string
	Value  float64
}

type Source interface {
	Baseline(selected generic.Selection) error // starting point
	Poll() error                  // sample
	Stop() error                  // ending point
	Summary() []Counter           // report
}
