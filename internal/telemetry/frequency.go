package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"goset/internal/generic"
)

type FreqSource struct {
	Root  string // generic.SysCPU in production, temp dir in tests
	cpus  generic.CPUSet
	files []*os.File
	buf   []byte
	min   []float64
	max   []float64
	sum   []float64
	n     []int
}

func (source *FreqSource) Baseline(cpus generic.CPUSet) error {
	source.cpus = cpus
	size := source.cpus.Max() + 1

	source.files = make([]*os.File, size)
	source.buf = make([]byte, 32)
	source.min = make([]float64, size)
	source.max = make([]float64, size)
	source.sum = make([]float64, size)
	source.n = make([]int, size)

	for cpu := range source.cpus.All() {
		path := filepath.Join(source.Root, fmt.Sprintf("cpu%d", cpu), generic.CpufreqDir, generic.ScalingCurFreq)
		file, err := os.Open(path)
		if err != nil {
			continue // absent on some cpus/VM
		}
		source.files[cpu] = file
		source.min[cpu] = -1
	}
	return nil // task not started: a sample here reads the pre-task clock
}

func (source *FreqSource) Poll() error { return source.sample() }

func (source *FreqSource) sample() error {
	for cpu := range source.cpus.All() {
		file := source.files[cpu]
		if file == nil {
			continue
		}
		if _, err := file.Seek(0, 0); err != nil {
			continue
		}
		n, err := file.Read(source.buf)
		if err != nil {
			continue
		}
		khz, err := strconv.ParseFloat(strings.TrimSpace(string(source.buf[:n])), 64)
		if err != nil {
			continue
		}
		hz := khz * 1000
		if source.min[cpu] < 0 || hz < source.min[cpu] {
			source.min[cpu] = hz
		}
		if hz > source.max[cpu] {
			source.max[cpu] = hz
		}
		source.sum[cpu] += hz
		source.n[cpu]++
	}
	return nil
}

func (source *FreqSource) Stop() error {
	for _, file := range source.files {
		if file != nil {
			if err := file.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (source *FreqSource) Summary() []Counter {
	out := make([]Counter, 0, source.cpus.Count()*3)
	for cpu := range source.cpus.All() {
		var low, high, mean float64
		if cpu < len(source.n) && source.n[cpu] > 0 {
			low, high = source.min[cpu], source.max[cpu]
			mean = source.sum[cpu] / float64(source.n[cpu])
		}
		out = append(out,
			Counter{Source: generic.SourceFreq, CPU: cpu, Name: generic.FreqMin, Value: low},
			Counter{Source: generic.SourceFreq, CPU: cpu, Name: generic.FreqMax, Value: high},
			Counter{Source: generic.SourceFreq, CPU: cpu, Name: generic.FreqAvg, Value: mean},
		)
	}
	return out
}
