package telemetry

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type IRQCount struct {
	Steerable    uint64 // numbered rows (kernel)
	NonSteerable uint64 // named rows: NMI, LOC, RES, CAL, TLB, ... (PCIE)
}


type IRQSource struct {
	cpus  []int
	start []IRQCount
	end   []IRQCount
	err   error
}


func (src *IRQSource) Baseline(target Target) error {
	src.cpus = target.BenchCPUs
	var err error
	src.start, err = ReadIRQCounts()
	return err
}


func (src *IRQSource) Poll() error { return nil }

func (src *IRQSource) Stop() error {
	src.end, src.err = ReadIRQCounts()
	return src.err
}


func (src *IRQSource) Summary() []Counter {
	if src.err != nil {
		return nil
	}
	out := make([]Counter, 0, len(src.cpus)*2)
	for _, cpu := range src.cpus {
		if cpu >= len(src.start) || cpu >= len(src.end) {
			continue
		}
		out = append(out,
			Counter{
				Source: "irq", CPU: cpu, Name: "soft",
				Value: float64(src.end[cpu].Steerable - src.start[cpu].Steerable),
			},
			Counter{
				Source: "irq", CPU: cpu, Name: "hard",
				Value: float64(src.end[cpu].NonSteerable - src.start[cpu].NonSteerable),
			},
		)
	}
	return out
}


func ReadIRQCounts() ([]IRQCount, error) {
	// harvest
	text, err := os.ReadFile("/proc/interrupts")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(text), "\n")

	// Parse per row
	header := strings.Fields(lines[0])
	ids := make([]int, 0, len(header)) // index: cpu id
	size := 0
	for _, field := range header {
		id, err := strconv.Atoi(strings.TrimPrefix(field, "CPU"))
		if err != nil {
			return nil, fmt.Errorf("/proc/interrupts: header %q: %w", field, err)
		}
		ids = append(ids, id)
		size = max(size, id+1)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("/proc/interrupts: no cpu columns")
	}

	// sums each cpu columns
	counts := make([]IRQCount, size) // index: cpu id
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) <= len(ids) {
			continue // ERR and MIS hold one value
		}

		// segmentation (non|steerable)
		label := strings.TrimSuffix(fields[0], ":")
		_, err := strconv.Atoi(label)
		steerable := err == nil

		// aggregate
		for col, id := range ids {
			val, err := strconv.ParseUint(fields[col+1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("/proc/interrupts: row %s: %w", label, err)
			}
			if steerable {
				counts[id].Steerable += val
			} else {
				counts[id].NonSteerable += val
			}
		}
	}
	return counts, nil
}
