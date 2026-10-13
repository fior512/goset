package isolation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"strconv"

	"goset/internal/cpu"
	"goset/internal/generic"
)




func (steering *Steering) ExpectedAffinities() map[string]string {
	if steering == nil {
		return nil
	}
	expected := make(map[string]string, len(steering.Saved))
	for label := range steering.Saved {
		expected[label] = steering.AppliedList
	}
	return expected
}

// https://www.kernel.org/doc/html/latest/core-api/irq/irq-affinity.html
func (steering *Steering) steerIRQs(topo *cpu.Topology, bench generic.CPUSet, housekeeper int) error {
	allowed := topo.Online
	allowed.AndNot(bench)
	allowed.ClearBit(housekeeper)
	if !allowed.Any() {
		return fmt.Errorf("no cpus left outside the benchmark set to receive irqs")
	}
	list := allowed.String()

	entries, err := os.ReadDir(generic.ProcIRQ)
	if err != nil {
		return err
	}
	steering.Errors = map[string]int{}
	steering.Saved = map[string]string{}
	steering.AppliedList = list
	for _, e := range entries {
		_, err := strconv.Atoi(e.Name());
		if !e.IsDir() || (err != nil) {
			continue
		}
		path := filepath.Join(generic.ProcIRQ, e.Name(), generic.SmpAffinityList)
		prev, err := generic.ReadFileTrim(path)
		if err != nil {
			continue
		}
		steering.Attempted++
		if err := os.WriteFile(path, []byte(list), 0o644); err != nil {
			steering.Rejected++
			steering.Errors[errnoLabel(err)]++
			continue
		}
		steering.Saved[e.Name()] = prev
		steering.Applied++
	}

	// check if writable
	for label := range steering.Saved {
		current, err := generic.ReadFileTrim(filepath.Join(generic.ProcIRQ, label, generic.EffectiveAffinityList)) //read-only
		if err != nil {
			current, err = generic.ReadFileTrim(filepath.Join(generic.ProcIRQ, label, generic.SmpAffinityList)) // read/write
			if err != nil {
				continue
			}
		}
		cpus, err := generic.ParseCPUList(current)
		if err != nil {
			continue
		}
		if cpus.Any() {
			var overlap generic.CPUSet
			overlap = cpus
			overlap.And(bench)
			if overlap.Any() {
				steering.Remaining = append(steering.Remaining, label)
			}
		}
	}
	sort.Strings(steering.Remaining)
	return nil
}

// RestoreIRQs restore smp_affinity_lists
// https://www.kernel.org/doc/html/latest/core-api/irq/irq-affinity.html
func (steering *Steering) RestoreIRQs() (restored, failed int) {
	if steering == nil {
		return 0, 0
	}
	for label, val := range steering.Saved {
		path := filepath.Join(generic.ProcIRQ, label, generic.SmpAffinityList)
		if err := os.WriteFile(path, []byte(val), 0o644); err != nil {
			failed++
			continue
		}
		restored++
	}
	return
}

func errnoLabel(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		if en, ok := pe.Err.(syscall.Errno); ok {
			return en.Error()
		}
	}
	return err.Error()
}
