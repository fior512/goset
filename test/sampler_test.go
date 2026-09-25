package integration

import (
	"testing"
	"time"

	"goset/internal/generic"
	"goset/internal/telemetry"
)

type recordingSource struct {
	baselined generic.CPUSet
}

func (src *recordingSource) Baseline(cpus generic.CPUSet) error {
	src.baselined = cpus
	return nil
}

func (src *recordingSource) Poll() error { return nil }
func (src *recordingSource) Stop() error { return nil }
func (src *recordingSource) Summary() []telemetry.Counter {
	return nil
}

func TestSamplerBaselinesSourcesWithItsCPUSet(t *testing.T) {
	var cpus generic.CPUSet
	cpus.SetBit(3)
	cpus.SetBit(5)
	src := &recordingSource{}
	sam := &telemetry.Sampler{Cpus: cpus, Interval: time.Millisecond, Sources: []telemetry.Source{src}}

	if err := sam.Start(func() error { return nil }); err != nil {
		t.Fatalf("Sampler.Start: %v", err)
	}
	sam.Stop()

	if src.baselined.String() != cpus.String() {
		t.Errorf("source baselined with cpus %q, want %q", src.baselined.String(), cpus.String())
	}
}
