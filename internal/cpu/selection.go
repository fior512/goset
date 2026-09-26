package cpu

import (
	"fmt"
	"strconv"

	"goset/internal/generic"
)

func SelectCPUs(topo *Topology, n int, include generic.CPUSet, exclude generic.CPUSet, numa int) (*generic.Selection, error) {
	candidates := topo.Online

	// incl&excl overlap check by run()
	candidates.AndNot(exclude) // exclude
	if !include.IsSubset(candidates) {
		return nil, fmt.Errorf("include %s: offline or excluded cpu",
			include.String())
	}

	// numa
	candidates, numa, err := constrainNuma(topo, candidates, include, numa)
	if err != nil {
		return nil, err
	}

	// minimum
	scope := " on any numa node"
	if numa >= 0 {
		scope = " on numa " + strconv.Itoa(numa)
	}
	if candidates.Count() < n+1 { // +housekeeper
		return nil, fmt.Errorf(
			"need %d cpu(s) plus 1 housekeeper but only %d candidate(s) remain%s",
			n, candidates.Count(), scope)
	}

	// evaluate
	scores, err := rankCPUs(topo, candidates, include)
	if err != nil {
		return nil, err
	}

	// selection
	var selection, cores generic.CPUSet
	for _, score := range scores[:n] {
		selection.SetBit(score.CPU)
		cores.SetBit(topo.Core[score.CPU])
	}

	// housekeeper: first non previsouly selected
	housekeeper := scores[n].CPU
	for _, score := range scores[n:] {
		if !cores.GetBit(topo.Core[score.CPU]) {
			housekeeper = score.CPU
			break
		}
	}

	return &generic.Selection{
		Scores:      scores,
		Numa:        numa,
		Task:        selection,
		HouseKeeper: housekeeper,
	}, nil
}
