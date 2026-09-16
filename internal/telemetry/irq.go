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


func (s *IRQSource) Baseline(t Target) error {
	s.cpus = t.BenchCPUs
	var err error
	s.start, err = ReadIRQCounts()
	return err
}


func (s *IRQSource) Poll() error { return nil }

func (s *IRQSource) Stop() error {
	s.end, s.err = ReadIRQCounts()
	return s.err
}


func (s *IRQSource) Summary() []Counter {
	if s.err != nil {
		return nil
	}
	out := make([]Counter, 0, len(s.cpus)*2)
	for _, c := range s.cpus {
		if c >= len(s.start) || c >= len(s.end) {
			continue
		}
		out = append(out,
			Counter{
				Source: "irq", CPU: c, Name: "soft",
				Value: float64(s.end[c].Steerable - s.start[c].Steerable),
			},
			Counter{
				Source: "irq", CPU: c, Name: "hard",
				Value: float64(s.end[c].NonSteerable - s.start[c].NonSteerable),
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
			v, err := strconv.ParseUint(fields[col+1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("/proc/interrupts: row %s: %w", label, err)
			}
			if steerable {
				counts[id].Steerable += v
			} else {
				counts[id].NonSteerable += v
			}
		}
	}
	return counts, nil
}
