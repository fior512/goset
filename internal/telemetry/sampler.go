package telemetry

import (
	"fmt"
	"runtime"
	"slices"
	"sync"
	"time"

	"goset/internal/generic"
)

type Sampler struct {
	Cpus     generic.CPUSet
	Interval time.Duration
	Sources  []Source

	exit     chan struct{}
	counters chan sampled
	once     sync.Once
	errs     []error
	polls    int
}

// sampled is what the housekeeper hands back when the run ends.
type sampled struct {
	counters []Counter
	errs     []error
	polls    int
}

func (sampler *Sampler) Start(pin func() error) error {
	sampler.exit = make(chan struct{})
	sampler.counters = make(chan sampled, 1)

	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := pin(); err != nil {
			ready <- fmt.Errorf("telemetry housekeeper pin: %w", err)
			return
		}

		if err := sampler.baseline(); err != nil {
			ready <- err
			return
		}
		ready <- nil

		sampler.poll()
		sampler.counters <- sampler.summary()
	}()
	return <-ready
}

func (sampler *Sampler) baseline() error {
	for _, source := range sampler.Sources {
		if err := source.Baseline(sampler.Cpus); err != nil {
			return fmt.Errorf("telemetry baseline %T: %w", source, err)
		}
	}
	return nil
}

func (sampler *Sampler) poll() {
	if sampler.Interval <= 0 {
		<-sampler.exit
		return
	}
	ticker := time.NewTicker(sampler.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-sampler.exit:
			return
		case <-ticker.C:
			for _, source := range sampler.Sources {
				sampler.recordFailure("poll", source, source.Poll())
			}
			sampler.polls++
		}
	}
}

func (sampler *Sampler) summary() sampled {
	out := sampled{polls: sampler.polls}
	for _, source := range sampler.Sources {
		sampler.recordFailure("stop", source, source.Stop())
		out.counters = append(out.counters, source.Summary()...)
	}
	out.errs = sampler.errs
	return out
}

// one entry per distinct failure: a source failing on every tick keeps one
func (sampler *Sampler) recordFailure(stage string, source Source, err error) {
	if err == nil {
		return
	}
	failure := fmt.Errorf("telemetry %s %T: %w", stage, source, err)
	if slices.ContainsFunc(sampler.errs, func(seen error) bool { return seen.Error() == failure.Error() }) {
		return
	}
	sampler.errs = append(sampler.errs, failure)
}

// the counters, the poll ticks taken, and every distinct source failure
func (sampler *Sampler) Stop() ([]Counter, int, []error) {
	sampler.once.Do(func() { close(sampler.exit) })
	final := <-sampler.counters
	return final.counters, final.polls, final.errs
}
