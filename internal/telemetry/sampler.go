package telemetry

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"goset/internal/generic"
)

type Sampler struct {
	Cpus     generic.CPUSet
	Interval time.Duration
	Sources  []Source

	exit     chan struct{}
	counters chan []Counter
	once     sync.Once
}

func (sam *Sampler) Start(pin func() error) error {
	sam.exit = make(chan struct{})
	sam.counters = make(chan []Counter, 1)

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
	selected := generic.Selection{Task: sam.Cpus}
	for _, src := range sam.Sources {
		if err := src.Baseline(selected); err != nil {
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
				_ = src.Poll()
			}
		}
	}
}

func (sam *Sampler) summary() []Counter {
	var counters []Counter
	for _, src := range sam.Sources {
		_ = src.Stop()
		counters = append(counters, src.Summary()...)
	}
	return counters
}

func (sam *Sampler) Stop() []Counter {
	sam.once.Do(func() { close(sam.exit) })
	return <-sam.counters
}
