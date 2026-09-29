package telemetry

import (
	"fmt"
	"goset/internal/generic"
	"os"
	"strconv"
	"strings"
)

// ReadThrottleCounts indexes counts by cpu id; readable holds the cpus whose counter parsed.
func ReadThrottleCounts(cpus generic.CPUSet) (counts []uint64, readable generic.CPUSet) {
	counts = make([]uint64, cpus.Max()+1)
	for cpu := range cpus.All() {
		path := fmt.Sprintf(
			generic.SysCPU+
				"/cpu%d/thermal_throttle/core_throttle_count", cpu)
		text, err := os.ReadFile(path)
		if err != nil {
			continue // absent on some cpus/VMs: not reported
		}
		count, err := strconv.ParseUint(strings.TrimSpace(string(text)), 10, 64)
		if err != nil {
			continue
		}
		counts[cpu] = count
		readable.SetBit(cpu)
	}
	return counts, readable
}


type ThrottleSource struct {
	cpus     generic.CPUSet
	start    []uint64
	end      []uint64
	readable generic.CPUSet // read at Baseline and at Stop
}

func (src *ThrottleSource) Baseline(cpus generic.CPUSet) error {
	src.cpus = cpus
	src.start, src.readable = ReadThrottleCounts(src.cpus)
	return nil
}

func (src *ThrottleSource) Poll() error { return nil }

func (src *ThrottleSource) Stop() error {
	var readable generic.CPUSet
	src.end, readable = ReadThrottleCounts(src.cpus)
	src.readable.And(readable)
	return nil
}


func (src *ThrottleSource) Summary() []Counter {
	out := make([]Counter, 0, src.readable.Count())
	for cpu := range src.readable.All() {
		out = append(out, Counter{
			Source: generic.SourceThrottle, CPU: cpu, Name: generic.ThrottleCount,
			Value: float64(src.end[cpu] - src.start[cpu]),
		})
	}
	return out
}
