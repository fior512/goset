package telemetry

import (
	"fmt"
	"goset/internal/generic"
	"os"
	"strconv"
	"strings"
)

func ReadThrottleCounts(cpus generic.CPUSet) ([]uint64, error) {
	size := 0
	for cpu := range cpus.All() {
		size = cpu + 1
	}
	out := make([]uint64, size)
	for cpu := range cpus.All() {
		path := fmt.Sprintf(
			generic.SysCPU+
				"/cpu%d/thermal_throttle/core_throttle_count", cpu)
		text, err := os.ReadFile(path)
		if err != nil {
			continue // absent on some cpus/VMs, leave 0
		}
		out[cpu], _ = strconv.ParseUint(strings.TrimSpace(string(text)), 10, 64)
	}
	return out, nil
}


type ThrottleSource struct {
	cpus  generic.CPUSet
	start []uint64
	end   []uint64
}

func (src *ThrottleSource) Baseline(selected generic.Selection) error {
	src.cpus = selected.Task
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
	out := make([]Counter, 0, src.cpus.Count())
	for cpu := range src.cpus.All() {
		if cpu >= len(src.start) || cpu >= len(src.end) {
			continue
		}
		out = append(out, Counter{
			Source: "throttle", CPU: cpu, Name: "count",
			Value: float64(src.end[cpu] - src.start[cpu]),
		})
	}
	return out
}
