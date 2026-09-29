package telemetry

import (
	"errors"
	"fmt"
	"goset/internal/generic"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ReadThrottleCounts indexes counts by cpu id; absent holds the cpus with no thermal_throttle in sysfs.
func ReadThrottleCounts(root string, cpus generic.CPUSet) (counts []uint64, absent generic.CPUSet) {
	counts = make([]uint64, cpus.Max()+1)
	for cpu := range cpus.All() {
		path := filepath.Join(root, fmt.Sprintf("cpu%d", cpu), "thermal_throttle", "core_throttle_count")
		text, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			absent.SetBit(cpu)
			continue
		}
		if err != nil {
			continue // availability unknown: reads 0
		}
		counts[cpu], _ = strconv.ParseUint(strings.TrimSpace(string(text)), 10, 64) // malformed: reads 0
	}
	return counts, absent
}


type ThrottleSource struct {
	Root  string // generic.SysCPU in production, temp dir in tests
	cpus  generic.CPUSet // task cpus minus the absent ones
	start []uint64
	end   []uint64
}

func (src *ThrottleSource) Baseline(cpus generic.CPUSet) error {
	var absent generic.CPUSet
	src.start, absent = ReadThrottleCounts(src.Root, cpus)
	src.cpus = cpus
	src.cpus.AndNot(absent)
	return nil
}

func (src *ThrottleSource) Poll() error { return nil }

func (src *ThrottleSource) Stop() error {
	var absent generic.CPUSet
	src.end, absent = ReadThrottleCounts(src.Root, src.cpus)
	src.cpus.AndNot(absent)
	return nil
}


func (src *ThrottleSource) Summary() []Counter {
	out := make([]Counter, 0, src.cpus.Count())
	for cpu := range src.cpus.All() {
		var delta uint64
		if src.end[cpu] >= src.start[cpu] { // end read 0: unknown at Stop
			delta = src.end[cpu] - src.start[cpu]
		}
		out = append(out, Counter{
			Source: generic.SourceThrottle, CPU: cpu, Name: generic.ThrottleCount,
			Value: float64(delta),
		})
	}
	return out
}
