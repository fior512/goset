package telemetry

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"goset/internal/generic"
)

type IRQCount struct {
	Steerable    uint64 // numbered rows (kernel)
	NonSteerable uint64 // named rows: NMI, LOC, RES, CAL, TLB, ... (PCIE)
}


type IRQSource struct {
	cpus  generic.CPUSet
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
	out := make([]Counter, 0, src.cpus.Count()*2)
	for cpu := range src.cpus.All() {
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
	text, err := os.ReadFile(generic.ProcInterrupts)
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
			return nil, fmt.Errorf("%s: header %q: %w",generic.ProcInterrupts , field, err)
		}
		ids = append(ids, id)
		size = max(size, id+1)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%s: no cpu columns", generic.ProcInterrupts)
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
				return nil, fmt.Errorf("%s: row %s: %w", generic.ProcInterrupts, label, err)
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
