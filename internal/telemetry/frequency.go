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

func (src *FreqSource) Baseline(cpus generic.CPUSet) error {
	src.cpus = cpus
	size := src.cpus.Max() + 1

	src.files = make([]*os.File, size)
	src.buf = make([]byte, 32)
	src.min = make([]float64, size)
	src.max = make([]float64, size)
	src.sum = make([]float64, size)
	src.n = make([]int, size)

	for cpu := range src.cpus.All() {
		path := filepath.Join(src.Root, fmt.Sprintf("cpu%d", cpu), generic.CpufreqDir, generic.ScalingCurFreq)
		file, err := os.Open(path)
		if err != nil {
			continue // absent on some cpus/VM
		}
		src.files[cpu] = file
		src.min[cpu] = -1
	}
	return nil // task not started: a sample here reads the pre-task clock
}

func (src *FreqSource) Poll() error { return src.sample() }

func (src *FreqSource) sample() error {
	for cpu := range src.cpus.All() {
		file := src.files[cpu]
		if file == nil {
			continue
		}
		if _, err := file.Seek(0, 0); err != nil {
			continue
		}
		n, err := file.Read(src.buf)
		if err != nil {
			continue
		}
		khz, err := strconv.ParseFloat(strings.TrimSpace(string(src.buf[:n])), 64)
		if err != nil {
			continue
		}
		hz := khz * 1000
		if src.min[cpu] < 0 || hz < src.min[cpu] {
			src.min[cpu] = hz
		}
		if hz > src.max[cpu] {
			src.max[cpu] = hz
		}
		src.sum[cpu] += hz
		src.n[cpu]++
	}
	return nil
}

func (src *FreqSource) Stop() error {
	for _, file := range src.files {
		if file != nil {
			if err := file.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (src *FreqSource) Summary() []Counter {
	out := make([]Counter, 0, src.cpus.Count()*3)
	for cpu := range src.cpus.All() {
		var low, high, mean float64
		if cpu < len(src.n) && src.n[cpu] > 0 {
			low, high = src.min[cpu], src.max[cpu]
			mean = src.sum[cpu] / float64(src.n[cpu])
		}
		out = append(out,
			Counter{Source: generic.SourceFreq, CPU: cpu, Name: generic.FreqMin, Value: low},
			Counter{Source: generic.SourceFreq, CPU: cpu, Name: generic.FreqMax, Value: high},
			Counter{Source: generic.SourceFreq, CPU: cpu, Name: generic.FreqAvg, Value: mean},
		)
	}
	return out
}
