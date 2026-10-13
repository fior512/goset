package integration

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"goset/internal/generic"
	"goset/internal/telemetry"
)

type recordingSource struct {
	baselined generic.CPUSet
	pollErr   error
	stopErr   error
}

func (source *recordingSource) Baseline(cpus generic.CPUSet) error {
	source.baselined = cpus
	return nil
}

func (source *recordingSource) Poll() error { return source.pollErr }
func (source *recordingSource) Stop() error  { return source.stopErr }
func (source *recordingSource) Summary() []telemetry.Counter {
	return nil
}

func TestSamplerBaselinesSourcesWithItsCPUSet(t *testing.T) {
	var cpus generic.CPUSet
	cpus.SetBit(3)
	cpus.SetBit(5)
	source := &recordingSource{}
	sampler := &telemetry.Sampler{Cpus: cpus, Interval: time.Millisecond, Sources: []telemetry.Source{source}}

	if err := sampler.Start(func() error { return nil }); err != nil {
		t.Fatalf("Sampler.Start: %v", err)
	}
	sampler.Stop()

	if source.baselined.String() != cpus.String() {
		t.Errorf("source baselined with cpus %q, want %q", source.baselined.String(), cpus.String())
	}
}

func TestSamplerHandsBackEverySourceFailureOnce(t *testing.T) {
	source := &recordingSource{pollErr: errors.New("poll failed"), stopErr: errors.New("stop failed")}
	sampler := &telemetry.Sampler{Interval: time.Millisecond, Sources: []telemetry.Source{source}}
	if err := sampler.Start(func() error { return nil }); err != nil {
		t.Fatalf("Sampler.Start: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	_, polls, errs := sampler.Stop()

	if polls < 2 {
		t.Fatalf("polls = %d, want 2 or more ticks so the poll failure repeats", polls)
	}
	want := []error{
		fmt.Errorf("telemetry poll %T: %w", source, source.pollErr),
		fmt.Errorf("telemetry stop %T: %w", source, source.stopErr),
	}
	if len(errs) != len(want) || !reflect.DeepEqual(errMessages(errs), errMessages(want)) {
		t.Errorf("errs = %q, want %q: one entry per distinct failure, the %d poll ticks share one",
			errMessages(errs), errMessages(want), polls)
	}
}

// the messages, DeepEqual walks the wrapped chain instead
func errMessages(errs []error) []string {
	out := make([]string, 0, len(errs))
	for _, err := range errs {
		out = append(out, err.Error())
	}
	return out
}

func TestSamplerSilentOnACleanRun(t *testing.T) {
	sampler := &telemetry.Sampler{Interval: time.Millisecond, Sources: []telemetry.Source{&recordingSource{}}}
	if err := sampler.Start(func() error { return nil }); err != nil {
		t.Fatalf("Sampler.Start: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, _, errs := sampler.Stop(); len(errs) != 0 {
		t.Errorf("errs = %q, want none", errs)
	}
}
