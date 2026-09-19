package telemetry

import (
	"fmt"
	"goset/internal/generic"
	"os"
	"strconv"
	"strings"
)

func ReadThrottleCounts(cpus []int) ([]uint64, error) {
	out := make([]uint64, len(cpus))
	for i, cpu := range cpus {
		path := fmt.Sprintf(
			generic.SysCPU + 
			"/cpu%d/thermal_throttle/core_throttle_count", cpu)
		text, err := os.ReadFile(path)
		if err != nil {
			continue // absent on some cpus/VMs, leave 0
		}
		out[i], _ = strconv.ParseUint(strings.TrimSpace(string(text)), 10, 64)
	}
	return out, nil
}


type ThrottleSource struct {
	cpus  []int
	start []uint64
	end   []uint64
}

func (src *ThrottleSource) Baseline(target Target) error {
	src.cpus = target.BenchCPUs
	var err error
	src.start, err = ReadThrottleCounts(src.cpus)
	return err
}

func (src *ThrottleSource) Poll() error { return nil }

func (src *ThrottleSource) Stop() error {
	var err error
	src.end, err = ReadThrottleCounts(src.cpus)
	return err
}


func (src *ThrottleSource) Summary() []Counter {
	out := make([]Counter, 0, len(src.cpus))
	for i, cpu := range src.cpus {
		out = append(out, Counter{
			Source: "throttle", CPU: cpu, Name: "count",
			Value: float64(src.end[i] - src.start[i]),
		})
	}
	return out
}
