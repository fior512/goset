package telemetry

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"goset/internal/generic"
)

// https://man7.org/linux/man-pages/man5/proc.5.html
type RunqueueSource struct {
	Root  string // "/proc" in production, temp dir in tests
	value float64
}

func (source *RunqueueSource) Baseline(generic.CPUSet) error {
	return nil //file doesnt exist yet
}

func (source *RunqueueSource) Poll() error {
	source.sample()
	return nil
}

func (source *RunqueueSource) sample() {
	total, ok := source.sumRunDelay()
	if !ok {
		return // mid-exit race: keep sticky value
	}
	source.value = total
}

func (source *RunqueueSource) Stop() error {
	source.sample() // task usually reaped already: keeps last Poll value
	return nil
}

func (source *RunqueueSource) Summary() []Counter {
	return []Counter{{
		Source: generic.SourceSched,
		CPU:    -1, // run-global, not per-cpu
		Name:   generic.SchedRunDelay,
		Value:  source.value,
	}}
}

func (source *RunqueueSource) sumRunDelay() (float64, bool) {
	// /proc/[self]/task/
	selfDir := filepath.Join(source.Root, strconv.Itoa(os.Getpid()), "task")
	entries, err := os.ReadDir(selfDir)
	if err != nil {
		return 0, false
	}
	found := false
	var total float64
	for _, entry := range entries { // /proc/[self]/task/*/children
		children, err := os.ReadFile(filepath.Join(selfDir, entry.Name(), "children"))
		if err != nil {
			continue
		}
		for _, field := range strings.Fields(string(children)) {
			pid := filepath.Join(source.Root, field, "task")
			threads, err := os.ReadDir(pid)
			if err != nil {
				continue
			}
			for _, thread := range threads {
				raw, err := os.ReadFile(filepath.Join(pid, thread.Name(), "schedstat"))
				if err != nil {
					continue // thread exited mid-scan
				}
				delay, ok := parseRunDelay(string(raw))
				if !ok {
					continue
				}
				total += delay
				found = true
			}
		}
	}
	return total, found
}

func parseRunDelay(content string) (float64, bool) {
	fields := strings.Fields(content)
	if len(fields) < 2 {
		return 0, false
	}
	delay, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, false
	}
	return delay, true
}

func CountRunqueue(counters []Counter) float64 {
	for _, counter := range counters {
		if counter.Source == generic.SourceSched && counter.Name == generic.SchedRunDelay {
			return counter.Value
		}
	}
	return 0
}
