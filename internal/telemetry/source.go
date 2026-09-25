package telemetry

import "goset/internal/generic"

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
