package isolation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"syscall"

	"goset/internal/cpu"
	"goset/internal/generic"
)


func isSteerableLabel(label string) bool {
	_, err := strconv.Atoi(label) // soft: digit, hard:letters
	return err == nil
}

func (steer *Steering) ExpectedAffinities() map[string]string {
	if steer == nil {
		return nil
	}
	expected := make(map[string]string, len(steer.Saved))
	for label := range steer.Saved {
		expected[label] = steer.AppliedList
	}
	return expected
}

// https://www.kernel.org/doc/html/latest/core-api/irq/irq-affinity.html
func (steer *Steering) steerIRQs(topo *cpu.Topology, bench generic.CPUSet, housekeeper int) error {
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
	steer.Errors = map[string]int{}
	steer.Saved = map[string]string{}
	steer.AppliedList = list
	for _, e := range entries {
		if !e.IsDir() || !isSteerableLabel(e.Name()) {
			continue
		}
		path := filepath.Join(generic.ProcIRQ, e.Name(), generic.SmpAffinityList)
		prev, err := readFileTrim(path)
		if err != nil {
			continue
		}
		steer.Attempted++
		if err := os.WriteFile(path, []byte(list), 0o644); err != nil {
			steer.Rejected++
			steer.Errors[errnoLabel(err)]++
			continue
		}
		steer.Saved[e.Name()] = prev
		steer.Applied++
	}

	// check if writable
	for label := range steer.Saved {
		s, err := readFileTrim(filepath.Join(generic.ProcIRQ, label, generic.EffectiveAffinityList)) //read-only
		if err != nil {
			s, err = readFileTrim(filepath.Join(generic.ProcIRQ, label, generic.SmpAffinityList)) // read/write
			if err != nil {
				continue
			}
		}
		cpus, err := generic.ParseCPUList(s)
		if err != nil {
			continue
		}
		if cpus.Any() {
			var overlap generic.CPUSet
			overlap = cpus
			overlap.And(bench)
			if overlap.Any() {
				steer.Remaining = append(steer.Remaining, label)
			}
		}
	}
	sort.Strings(steer.Remaining)
	return nil
}

// RestoreIRQs restore smp_affinity_lists
// https://www.kernel.org/doc/html/latest/core-api/irq/irq-affinity.html
func (steer *Steering) RestoreIRQs() (restored, failed int) {
	if steer == nil {
		return 0, 0
	}
	for label, val := range steer.Saved {
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
