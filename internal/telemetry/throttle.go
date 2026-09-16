package telemetry

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func ReadThrottleCounts(cpus []int) ([]uint64, error) {
	out := make([]uint64, len(cpus))
	for i, c := range cpus {
		path := fmt.Sprintf(
			"/sys/devices/system/cpu/cpu%d/thermal_throttle/core_throttle_count", c)
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

func (s *ThrottleSource) Baseline(t Target) error {
	s.cpus = t.BenchCPUs
	var err error
	s.start, err = ReadThrottleCounts(s.cpus)
	return err
}

func (s *ThrottleSource) Poll() error { return nil }

func (s *ThrottleSource) Stop() error {
	var err error
	s.end, err = ReadThrottleCounts(s.cpus)
	return err
}


func (s *ThrottleSource) Summary() []Counter {
	out := make([]Counter, 0, len(s.cpus))
	for i, c := range s.cpus {
		out = append(out, Counter{
			Source: "throttle", CPU: c, Name: "count",
			Value: float64(s.end[i] - s.start[i]),
		})
	}
	return out
}
