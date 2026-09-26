package cpu

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"goset/internal/generic"
)

func constrainNuma(topo *Topology, candidates, include generic.CPUSet, numa int) (generic.CPUSet, int, error) {
	if numa == -2 {
		return candidates, -1, nil
	}

	onNode := func(node int) generic.CPUSet {
		var set generic.CPUSet
		for cpu := range candidates.All() {
			if topo.Numa[cpu] == node {
				set.SetBit(cpu)
			}
		}
		return set
	}

	/* host */
	host := []int{}
	for cpu := range topo.Online.All() {
		if numa := topo.Numa[cpu]; numa >= 0 && !slices.Contains(host, numa) {
			host = append(host, numa)
		}
	}
	slices.Sort(host)

	/* resolve */
	resolved := numa
	switch {
	case len(host) == 0 && resolved == -1:
		return candidates, -1, nil
	case len(host) == 0:
		return generic.CPUSet{}, 0, fmt.Errorf(
			"-numa %d: host reports no numa node", resolved)
	case resolved == -1:
		widest := 0
		for _, node := range host {
			held := onNode(node)
			if held.Count() > widest {
				widest, resolved = held.Count(), node
			}
		}
	case !slices.Contains(host, resolved):
		names := make([]string, 0, len(host))
		for _, node := range host {
			names = append(names, strconv.Itoa(node))
		}
		return generic.CPUSet{}, 0, fmt.Errorf(
			"-numa %d: host numa nodes %s", resolved, strings.Join(names, ", "))
	}

	/* include */
	kept := onNode(resolved)
	if !include.IsSubset(kept) {
		offNode := include
		offNode.AndNot(kept)
		return generic.CPUSet{}, 0, fmt.Errorf(
			"-numa %d: include %s: offline, excluded, or on another numa node",
			resolved, offNode.String())
	}
	return kept, resolved, nil
}
