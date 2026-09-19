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

type SteerResult struct {
	Attempted int
	Applied   int
	Rejected  int
	Errors    map[string]int    // errno string -> count
	Remaining []string          // IRQ labels still permitting a benchmark CPU
	Saved     map[string]string // IRQ label -> original smp_affinity_list
}

func isSteerableLabel(label string) bool {
	_, err := strconv.Atoi(label) // soft: digit, hard:letters
	return err == nil
}

// https://www.kernel.org/doc/html/latest/core-api/irq/irq-affinity.html
func SteerIRQs(topo *cpu.Topology, bench generic.CPUSet, housekeeper int) (*SteerResult, error) {
	allowed := topo.Online
	allowed.AndNot(bench)
	allowed.ClearBit(housekeeper)
	if !allowed.Any() {
		return nil, fmt.Errorf("no cpus left outside the benchmark set to receive irqs")
	}
	list := allowed.String()

	entries, err := os.ReadDir(generic.ProcIRQ)
	if err != nil {
		return nil, err
	}
	res := &SteerResult{Errors: map[string]int{}, Saved: map[string]string{}}
	for _, e := range entries {
		if !e.IsDir() || !isSteerableLabel(e.Name()) {
			continue
		}
		path := filepath.Join(generic.ProcIRQ, e.Name(), generic.SmpAffinityList)
		prev, err := readFileTrim(path)
		if err != nil {
			continue
		}
		res.Attempted++
		if err := os.WriteFile(path, []byte(list), 0o644); err != nil {
			res.Rejected++
			res.Errors[errnoLabel(err)]++
			continue
		}
		res.Saved[e.Name()] = prev
		res.Applied++
	}

	// check if writable
	for label := range res.Saved {
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
				res.Remaining = append(res.Remaining, label)
			}
		}
	}
	sort.Strings(res.Remaining)
	return res, nil
}

// RestoreIRQs restore smp_affinity_lists
// https://www.kernel.org/doc/html/latest/core-api/irq/irq-affinity.html
func RestoreIRQs(r *SteerResult) (restored, failed int) {
	if r == nil {
		return 0, 0
	}
	for label, val := range r.Saved {
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
