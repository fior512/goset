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

func (src *IRQDriftSource) Baseline(generic.CPUSet) error {
	src.sample()
	return nil
}

func (src *IRQDriftSource) Poll() error {
	src.sample()
	return nil
}

func (src *IRQDriftSource) Stop() error {
	src.sample()
	return nil
}

func (src *IRQDriftSource) sample() {
	for label, want := range src.Expected {
		path := filepath.Join(src.Root, label, generic.SmpAffinityList)
		data, err := os.ReadFile(path)
		if err != nil {
			src.Drifted[label] = true // IRQ vanished or unreadable: state not held
			continue
		}
		got := strings.TrimSpace(string(data))
		if !generic.CPUListEqual(want, got) {
			src.Drifted[label] = true
		}
	}
}

func (src *IRQDriftSource) Summary() []Counter {
	return []Counter{{
		Source: generic.SourceIRQSteer,
		CPU:    -1, // run-global, not per-cpu
		Name:   generic.SteerDrift,
		Value:  float64(len(src.Drifted)),
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
