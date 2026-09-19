package cpu

import (
	"fmt"
	"goset/internal/generic"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Topology follows `/sys/devices/system/cpu/`
type Topology struct {
	Online     generic.CPUSet // availabe threads
	Core       []int          // index: cpu id
	NumaNode   []int          // index: cpu id
	KernelIsol generic.CPUSet
	NohzFull   generic.CPUSet
	RcuNocb    generic.CPUSet

	Driver   []string // index: cpu id, cpufreq/scaling_driver
	Governor []string // index: cpu id, cpufreq/scaling_governor
	EPP      []string // index: cpu id, cpufreq/energy_performance_preference
	MinFreq  []int    // index: cpu id, cpufreq/scaling_min_freq, kHz
	MaxFreq  []int    // index: cpu id, cpufreq/scaling_max_freq, kHz
}

// readFileTrim reads a file and trims surrounding whitespace.
func readFileTrim(path string) (string, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(text)), nil
}

func GetTopology() (*Topology, error) {
	// https://docs.kernel.org/core-api/cpu_hotplug.html
	online, err := readCPUList(generic.SysCPU + "/online")
	if err != nil {
		return nil, err
	}
	size := online.Max()+1
	topo := &Topology{
		Online:   online,
		Core:     make([]int, size),
		NumaNode: make([]int, size),
		Driver:   make([]string, size),
		Governor: make([]string, size),
		EPP:      make([]string, size),
		MinFreq:  make([]int, size),
		MaxFreq:  make([]int, size),
	}

	///Siblings
	// https://docs.kernel.org/admin-guide/cputopology.html
	for cpu := range online.All() {
		siblings, err := readCPUList(fmt.Sprintf(
			generic.SysCPU+"/cpu%d/topology/thread_siblings_list",
			cpu))
		topo.Core[cpu] = cpu
		if first := siblings.NextSet(0); err == nil && first >= 0 {
			topo.Core[cpu] = first
		}
		topo.NumaNode[cpu] = -1
	}

	///Cpufreq
	for cpu := range online.All() {
		// https://docs.kernel.org/admin-guide/pm/cpufreq.html
		base := fmt.Sprintf(generic.SysCPU+"/cpu%d/cpufreq", cpu)
		topo.Driver[cpu], _ = readFileTrim(filepath.Join(base, "scaling_driver"))
		topo.Governor[cpu], _ = readFileTrim(filepath.Join(base, "scaling_governor"))
		topo.EPP[cpu], _ = readFileTrim(filepath.Join(base, "energy_performance_preference"))
		if text, err := readFileTrim(filepath.Join(base, "scaling_min_freq")); err == nil {
			topo.MinFreq[cpu], _ = strconv.Atoi(text)
		}
		if text, err := readFileTrim(filepath.Join(base, "scaling_max_freq")); err == nil {
			topo.MaxFreq[cpu], _ = strconv.Atoi(text)
		}
	}

	///NumaNode
	// https://www.kernel.org/doc/Documentation/ABI/stable/sysfs-devices-node
	lists, err := filepath.Glob(generic.SysNode + "/node*/cpulist")
	if err != nil {
		return nil, err
	}
	for _, path := range lists {
		node, err := strconv.Atoi(
			strings.TrimPrefix(filepath.Base(filepath.Dir(path)), "node"))
		if err != nil {
			continue
		}
		cpus, err := readCPUList(path)
		if err != nil {
			continue
		}
		cpus.And(online) // cpulist may hold offline cpus
		for cpu := range cpus.All() {
			topo.NumaNode[cpu] = node
		}
	}

	///KernelIsol & NohzFull
	// https://www.kernel.org/doc/html/latest/admin-guide/kernel-parameters.html
	topo.KernelIsol, _ = readCPUList(generic.SysCPU + "/isolated") // absent/null if not used
	topo.NohzFull, _ = readCPUList(generic.SysCPU + "/nohz_full")

	///RcuNocb
	topo.RcuNocb, _ = readCmdlineCPUList("rcu_nocbs") // absent if not used
	return topo, nil
}

// readCmdlineCPUList extracts a "<param>=<cpulist>" token from /proc/cmdline
// https://www.kernel.org/doc/html/latest/admin-guide/kernel-parameters.html
func readCmdlineCPUList(param string) (generic.CPUSet, error) {
	text, err := os.ReadFile(generic.ProcCmd)
	if err != nil {
		return generic.CPUSet{}, err
	}
	prefix := param + "="
	for _, field := range strings.Fields(string(text)) {
		if list, ok := strings.CutPrefix(field, prefix); ok {
			return generic.ParseCPUList(list)
		}
	}
	return generic.CPUSet{}, nil // param absent
}

// readCPUList wrapper for read+ParseCPUList
func readCPUList(path string) (generic.CPUSet, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return generic.CPUSet{}, err
	}
	set, err := generic.ParseCPUList(string(text))
	if err != nil {
		return generic.CPUSet{}, fmt.Errorf("%s: %w", path, err)
	}
	return set, nil
}
