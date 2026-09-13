package cpu

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"goset/internal/utils"
)

// ListCpus translate user string as mask
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
			a, err := strconv.Atoi(strings.TrimSpace(lo))
			if err != nil {
				return CPUSet{}, err
			}
			b, err := strconv.Atoi(strings.TrimSpace(hi))
			if err != nil {
				return CPUSet{}, err
			}

			// handle ranges and gremlins
			for id := min(a, b); id <= max(a, b); id++ {
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


// `/sys/devices/system/cpu/`
// maps use CPUSet.All()
type Topology struct {
	Online     CPUSet // avaialbe threads
	Siblings   map[int]CPUSet
	NumaNode   map[int]int
	KernalIsol map[int]bool
	NohzFull   map[int]bool

	HasIsolated bool
	HasNohzFull bool
}


func GetTopology() (*Topology, error) {
	topo := &Topology{
		Siblings: map[int]CPUSet{},
		NumaNode: map[int]int{},
	}

	// online
	cpus, err := utils.ReadFileTrim("/sys/devices/system/cpu/online")
	if err != nil {
		return nil, fmt.Errorf("read online cpus: %w", err)
	}
	if topo.Online, err = ParseCPUList(cpus); err != nil {
		return nil, fmt.Errorf("parse online cpus: %w", err)
	}

	// siblings & NumaNode
	for cpu := range topo.Online.All() {
		folder := fmt.Sprintf("/sys/devices/system/cpu/cpu%d", cpu)
		if sib, err := utils.ReadFileTrim(folder + "/topology/threads_siblings_list"); err == nil {
			if siblings, err := ParseCPUList(sib); err == nil {
				topo.Siblings[cpu] = siblings
			}
		}

		if _, ok := topo.Siblings[cpu]; !ok {
			var tmp CPUSet
			tmp.SetBit(cpu)
			topo.Siblings[cpu] = tmp
		}

		topo.NumaNode[cpu] = -1
		if entries, err := filepath.Glob(folder + "/node*"); err == nil {
			for _, e := range entries {
				name := filepath.Base(e)
				if n, err := strconv.Atoi(strings.TrimPrefix(name, "node")); err == nil {
					topo.NumaNode[cpu] = n
					break
				}
			}
		}
	}

	//Isolated & Nohzfull
	topo.KernalIsol = readCPUListFile("/sys/devices/system/cpu/isolated")
	topo.HasIsolated = len(topo.KernalIsol) > 0
	topo.NohzFull = readCPUListFile("sys/devices/system/cpu/nohz_full")
	topo.HasNohzFull = len(topo.NohzFull) > 0

	return topo, nil
}


func readCPUListFile(path string) map[int]bool {
	m := map[int]bool{}
	list, err := utils.ReadFileTrim(path)
	if err != nil {
		return m
	}

	set, err := ParseCPUList(list)
	if err != nil {
		return m
	}

	for c := range set.All() {
		m[c] = true
	}
	return m
}
