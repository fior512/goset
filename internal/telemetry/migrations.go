package telemetry

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"goset/internal/generic"
)

// https://man7.org/linux/man-pages/man5/proc.5.html
type MigrationsSource struct {
	Root  string // "/proc" in production, temp dir in tests
	value float64
}

func (src *MigrationsSource) Baseline(generic.CPUSet) error {
	return nil //file doesnt exist yet
}

func (src *MigrationsSource) Poll() error {
	src.sample()
	return nil
}

func (src *MigrationsSource) Stop() error {
	src.sample() // task usually reaped already: keeps last Poll value
	return nil
}

func (src *MigrationsSource) Summary() []Counter {
	return []Counter{{
		Source: generic.SourceSched,
		CPU:    -1, // run-global, not per-cpu
		Name:   generic.SchedMigrations,
		Value:  src.value,
	}}
}

func (src *MigrationsSource) sample() {
	total, ok := src.readChildrenMigrations()
	if !ok {
		return
	}
	src.value = total
}

func (src *MigrationsSource) readChildrenMigrations() (float64, bool) {
	var total float64
	found := false
	for _, pid := range src.childPIDs() {
		if value, ok := src.sumThreads(pid); ok {
			total += value
			found = true
		}
	}
	return total, found
}

func (src *MigrationsSource) childPIDs() []int {
	// /proc/[self]/task/
	taskDir := filepath.Join(src.Root, strconv.Itoa(os.Getpid()), "task")
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		return nil
	}
	var pids []int
	for _, entry := range entries { // /proc/[self]/task/*/children
		data, err := os.ReadFile(filepath.Join(taskDir, entry.Name(), "children"))
		if err != nil {
			continue
		}
		for _, field := range strings.Fields(string(data)) {
			pid, err := strconv.Atoi(field)
			if err != nil {
				continue
			}
			pids = append(pids, pid)
		}
	}
	return pids
}

func (src *MigrationsSource) sumThreads(pid int) (float64, bool) {
	taskDir := filepath.Join(src.Root, strconv.Itoa(pid), "task")
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		return 0, false
	}
	var total float64
	found := false
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(taskDir, entry.Name(), "sched"))
		if err != nil {
			continue // thread exited mid-scan
		}
		if value, ok := parseNrMigrations(string(data)); ok {
			total += value
			found = true
		}
	}
	return total, found
}

func parseNrMigrations(sched string) (float64, bool) {
	for _, line := range strings.Split(sched, "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) != "se.nr_migrations" {
			continue
		}
		count, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return 0, false
		}
		return count, true
	}
	return 0, false
}

func CountMigrations(counters []Counter) int {
	for _, counter := range counters {
		if counter.Source == generic.SourceSched && counter.Name == generic.SchedMigrations {
			return int(counter.Value)
		}
	}
	return 0
}
