package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"goset/internal/generic"
)

type IRQCount struct {
	Steerable    uint64 // numbered rows (kernel)
	NonSteerable uint64 // named rows: NMI, LOC, RES, CAL, TLB, ... (PCIE)
}

type IRQSource struct {
	Root  string // generic.ProcRoot in production, temp dir in tests
	cpus  generic.CPUSet
	start []IRQCount
	end   []IRQCount
}

func (src *IRQSource) Baseline(cpus generic.CPUSet) error {
	src.cpus = cpus
	var err error
	src.start, err = ReadIRQCounts(src.Root)
	return err
}

func (src *IRQSource) Poll() error { return nil }

func (src *IRQSource) Stop() error {
	end, err := ReadIRQCounts(src.Root)
	src.end = end
	return err
}

// A cpu the file holds no column for, and a failed end read, both read 0
func (src *IRQSource) Summary() []Counter {
	out := make([]Counter, 0, src.cpus.Count()*2)
	for cpu := range src.cpus.All() {
		count := IRQCount{}
		if cpu < len(src.start) && cpu < len(src.end) {
			count.Steerable = src.end[cpu].Steerable - src.start[cpu].Steerable
			count.NonSteerable = src.end[cpu].NonSteerable - src.start[cpu].NonSteerable
		}
		out = append(out,
			Counter{
				Source: generic.SourceIRQ, CPU: cpu, Name: generic.IRQSteerable,
				Value: float64(count.Steerable),
			},
			Counter{
				Source: generic.SourceIRQ, CPU: cpu, Name: generic.IRQNonSteerable,
				Value: float64(count.NonSteerable),
			},
		)
	}
	return out
}

// https://man7.org/linux/man-pages/man5/proc.5.html
func ReadIRQCounts(root string) ([]IRQCount, error) {
	path := filepath.Join(root, generic.ProcInterruptsName)

	// harvest
	text, err := os.ReadFile(path)
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
			return nil, fmt.Errorf("%s: header %q: %w", path, field, err)
		}
		ids = append(ids, id)
		size = max(size, id+1)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%s: no cpu columns", path)
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
				return nil, fmt.Errorf("%s: row %s: %w", path, label, err)
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
