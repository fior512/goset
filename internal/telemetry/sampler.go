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

func (sam *Sampler) Start(pin func() error) error {
	sam.exit = make(chan struct{})
	sam.counters = make(chan sampled, 1)

	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := pin(); err != nil {
			ready <- fmt.Errorf("telemetry housekeeper pin: %w", err)
			return
		}

		if err := sam.baseline(); err != nil {
			ready <- err
			return
		}
		ready <- nil

		sam.poll()
		sam.counters <- sam.summary()
	}()
	return <-ready
}

func (sam *Sampler) baseline() error {
	for _, src := range sam.Sources {
		if err := src.Baseline(sam.Cpus); err != nil {
			return fmt.Errorf("telemetry baseline %T: %w", src, err)
		}
	}
	return nil
}

func (sam *Sampler) poll() {
	if sam.Interval <= 0 {
		<-sam.exit
		return
	}
	ticker := time.NewTicker(sam.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-sam.exit:
			return
		case <-ticker.C:
			for _, src := range sam.Sources {
				sam.recordFailure("poll", src, src.Poll())
			}
			sam.polls++
		}
	}
}

func (sam *Sampler) summary() sampled {
	out := sampled{polls: sam.polls}
	for _, src := range sam.Sources {
		sam.recordFailure("stop", src, src.Stop())
		out.counters = append(out.counters, src.Summary()...)
	}
	out.errs = sam.errs
	return out
}

// one entry per distinct failure: a source failing on every tick keeps one
func (sam *Sampler) recordFailure(stage string, src Source, err error) {
	if err == nil {
		return
	}
	failure := fmt.Errorf("telemetry %s %T: %w", stage, src, err)
	if slices.ContainsFunc(sam.errs, func(seen error) bool { return seen.Error() == failure.Error() }) {
		return
	}
	sam.errs = append(sam.errs, failure)
}

// the counters, the poll ticks taken, and every distinct source failure
func (sam *Sampler) Stop() ([]Counter, int, []error) {
	sam.once.Do(func() { close(sam.exit) })
	final := <-sam.counters
	return final.counters, final.polls, final.errs
}
