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

func (source *IRQSource) Baseline(cpus generic.CPUSet) error {
	source.cpus = cpus
	var err error
	source.start, err = ReadIRQCounts(source.Root)
	return err
}

func (source *IRQSource) Poll() error { return nil }

func (source *IRQSource) Stop() error {
	end, err := ReadIRQCounts(source.Root)
	source.end = end
	return err
}

// A cpu the file holds no column for, and a failed end read, both read 0
func (source *IRQSource) Summary() []Counter {
	out := make([]Counter, 0, source.cpus.Count()*2)
	for cpu := range source.cpus.All() {
		count := IRQCount{}
		if cpu < len(source.start) && cpu < len(source.end) {
			count.Steerable = source.end[cpu].Steerable - source.start[cpu].Steerable
			count.NonSteerable = source.end[cpu].NonSteerable - source.start[cpu].NonSteerable
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
		isSteerable := err==nil

		// aggregate
		for col, id := range ids {
			val, err := strconv.ParseUint(fields[col+1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%s: row %s: %w", path, label, err)
			}
			if isSteerable {
				counts[id].Steerable += val
			} else {
				counts[id].NonSteerable += val
			}
		}
	}
	return counts, nil
}
