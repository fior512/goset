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

func (src *recordingSource) Baseline(cpus generic.CPUSet) error {
	src.baselined = cpus
	return nil
}

func (src *recordingSource) Poll() error { return src.pollErr }
func (src *recordingSource) Stop() error  { return src.stopErr }
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

func TestSamplerHandsBackEverySourceFailureOnce(t *testing.T) {
	src := &recordingSource{pollErr: errors.New("poll failed"), stopErr: errors.New("stop failed")}
	sam := &telemetry.Sampler{Interval: time.Millisecond, Sources: []telemetry.Source{src}}
	if err := sam.Start(func() error { return nil }); err != nil {
		t.Fatalf("Sampler.Start: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	_, polls, errs := sam.Stop()

	if polls < 2 {
		t.Fatalf("polls = %d, want 2 or more ticks so the poll failure repeats", polls)
	}
	want := []error{
		fmt.Errorf("telemetry poll %T: %w", src, src.pollErr),
		fmt.Errorf("telemetry stop %T: %w", src, src.stopErr),
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
	sam := &telemetry.Sampler{Interval: time.Millisecond, Sources: []telemetry.Source{&recordingSource{}}}
	if err := sam.Start(func() error { return nil }); err != nil {
		t.Fatalf("Sampler.Start: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, _, errs := sam.Stop(); len(errs) != 0 {
		t.Errorf("errs = %q, want none", errs)
	}
}
