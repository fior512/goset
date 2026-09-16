package telemetry

import (
	"fmt"
	"runtime"
	"time"
)

type Sampler struct {
	Cpus     []int
	Interval time.Duration
	Sources  []Source
	Exit     chan struct{} // message exit
	Done     chan struct{} // confirm exit
}


func (sam *Sampler) Start(pin func() error) error {
	target := Target{BenchCPUs: sam.Cpus}
	for _, src := range sam.Sources {
		if err := src.Baseline(target); err != nil {
			return fmt.Errorf("telemetry baseline %s: %w", "irq", err)
		}
	}
	go sam.run(pin)
	return nil
}


func (sam *Sampler) run(pin func() error) {
	defer close(sam.Done)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	_ = pin()

	if sam.Interval <= 0 {
		<-sam.Exit
		return
	}
	ticker := time.NewTicker(sam.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-sam.Exit:
			return
		case <-ticker.C:
			for _, src := range sam.Sources {
				_ = src.Poll()
			}
		}
	}
}


func (sam *Sampler) Stop() []Counter {
	close(sam.Exit)
	<-sam.Done
	var counters []Counter
	for _, src := range sam.Sources {
		_ = src.Stop()
		counters = append(counters, src.Summary()...)
	}
	return counters
}
