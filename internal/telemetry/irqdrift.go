package telemetry

import (
	"os"
	"path/filepath"
	"strings"

	"goset/internal/generic"
)

// https://www.kernel.org/doc/html/latest/core-api/irq/irq-affinity.html
type IRQDriftSource struct {
	Root     string
	Expected map[string]string // IRQ label -> applied affinity list
	Drifted  map[string]bool   // sticky: once drifted, stays counted
}

func (source *IRQDriftSource) Baseline(generic.CPUSet) error {
	source.sample()
	return nil
}

func (source *IRQDriftSource) Poll() error {
	source.sample()
	return nil
}

func (source *IRQDriftSource) Stop() error {
	source.sample()
	return nil
}

func (source *IRQDriftSource) sample() {
	for label, want := range source.Expected {
		path := filepath.Join(source.Root, label, generic.SmpAffinityList)
		data, err := os.ReadFile(path)
		if err != nil {
			source.Drifted[label] = true // IRQ vanished or unreadable: state not held
			continue
		}
		got := strings.TrimSpace(string(data))
		if !generic.CPUListEqual(want, got) {
			source.Drifted[label] = true
		}
	}
}

func (source *IRQDriftSource) Summary() []Counter {
	return []Counter{{
		Source: generic.SourceIRQSteer,
		CPU:    -1, // run-global, not per-cpu
		Name:   generic.SteerDrift,
		Value:  float64(len(source.Drifted)),
	}}
}

func CountDriftedIRQs(counters []Counter) int {
	for _, counter := range counters {
		if counter.Source == generic.SourceIRQSteer && counter.Name == generic.SteerDrift {
			return int(counter.Value)
		}
	}
	return 0
}
