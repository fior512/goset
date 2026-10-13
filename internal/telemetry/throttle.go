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
	Root  string         // generic.SysCPU in production, temp dir in tests
	all   generic.CPUSet // task cpus
	cpus  generic.CPUSet // task cpus minus the absent ones
	start []uint64
	end   []uint64
}

func (source *ThrottleSource) Baseline(cpus generic.CPUSet) error {
	var absent generic.CPUSet
	source.start, absent = ReadThrottleCounts(source.Root, cpus)
	source.all = cpus
	source.cpus = cpus
	source.cpus.AndNot(absent)
	return nil
}

func (source *ThrottleSource) Poll() error { return nil }

func (source *ThrottleSource) Stop() error {
	var absent generic.CPUSet
	source.end, absent = ReadThrottleCounts(source.Root, source.cpus)
	source.cpus.AndNot(absent)
	return nil
}

func (source *ThrottleSource) Summary() []Counter {
	out := make([]Counter, 0, source.all.Count())
	for cpu := range source.all.All() {
		var delta uint64
		if source.cpus.GetBit(cpu) && source.end[cpu] >= source.start[cpu] { // end read 0: unknown at Stop
			delta = source.end[cpu] - source.start[cpu]
		}
		out = append(out, Counter{
			Source: generic.SourceThrottle, CPU: cpu, Name: generic.ThrottleCount,
			Value: float64(delta),
		})
	}
	return out
}
