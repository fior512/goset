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
	t := time.NewTicker(sam.Interval)
	defer t.Stop()
	for {
		select {
		case <-sam.Exit:
			return
		case <-t.C:
			for _, src := range sam.Sources {
				_ = src.Poll()
			}
		}
	}
}


func (sam *Sampler) Stop() []string {
	close(sam.Exit)
	<-sam.Done
	summaries := make([]string, 0, len(sam.Sources))
	for _, src := range sam.Sources {
		_ = src.Stop()
		summaries = append(summaries, src.Summary())
	}
	return summaries
}
