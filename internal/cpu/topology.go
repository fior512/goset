package cpu

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ParseCPUList translate strings to CPUSet
func ParseCPUList(list string) (CPUSet, error) {
	str := strings.TrimSpace(list)
	if str == "" {
		return CPUSet{}, nil
	}

	var out CPUSet
	for _, element := range strings.Split(str, ",") {
		if element = strings.TrimSpace(element); element == "" {
			continue
		}

		if lo, hi, ok := strings.Cut(element, "-"); ok {
			l, err := strconv.Atoi(strings.TrimSpace(lo))
			if err != nil {
				return CPUSet{}, err
			}
			h, err := strconv.Atoi(strings.TrimSpace(hi))
			if err != nil {
				return CPUSet{}, err
			}

			// ranges and gremlins
			for id := min(l, h); id <= max(l, h); id++ {
				out.SetBit(id)
			}
		} else {
			id, err := strconv.Atoi(element)
			if err != nil {
				return CPUSet{}, err
			}
			out.SetBit(id)
		}
	}
	return out, nil
}


// Topology follows `/sys/devices/system/cpu/`
type Topology struct {
	Online     CPUSet // availabe threads
	Core       []int  // index: cpu id, value: lowest thread id on that core
	NumaNode   []int  // index: cpu id
	KernelIsol CPUSet
	NohzFull   CPUSet
}


func GetTopology() (*Topology, error) {
	online, err := readCPUList("/sys/devices/system/cpu/online")
	if err != nil {
		return nil, err
	}
	size := 0 // highest online cpu id + 1
	for cpu := range online.All() {
		size = cpu + 1
	}
	topo := &Topology{
		Online:   online,
		Core:     make([]int, size),
		NumaNode: make([]int, size),
	}

	///Siblings
	for cpu := range online.All() {
		siblings, err := readCPUList(fmt.Sprintf(
			"/sys/devices/system/cpu/cpu%d/topology/thread_siblings_list",
			cpu))
		topo.Core[cpu] = cpu
		if first := siblings.NextSet(0); err == nil && first >= 0 {
			topo.Core[cpu] = first
		}
		topo.NumaNode[cpu] = -1
	}

	///NumaNode
	lists, err := filepath.Glob("/sys/devices/system/node/node*/cpulist")
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
		for c := range cpus.All() {
			topo.NumaNode[c] = node
		}
	}

	///KernelIsol & NohzFull
	topo.KernelIsol, _ = readCPUList("/sys/devices/system/cpu/isolated") // absent/null if not used
	topo.NohzFull, _ = readCPUList("/sys/devices/system/cpu/nohz_full")
	return topo, nil
}


// readCPUList wrapper for read+ParseCPUList
func readCPUList(path string) (CPUSet, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return CPUSet{}, err
	}
	set, err := ParseCPUList(string(text))
	if err != nil {
		return CPUSet{}, fmt.Errorf("%s: %w", path, err)
	}
	return set, nil
}
