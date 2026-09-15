package cpu

import (
	"fmt"
)

// SelectionResult Benchmark Results
type SelectionResult struct {
	Benchmark   CPUSet
	HouseKeeper int
	Scores      []CPUScore
}


func SelectCPUs(topo *Topology, n int, include CPUSet, exclude CPUSet, NumaNode int) (*SelectionResult, error) {
	//TODO: impl NumaNode
	candidates := topo.Online

	// incl&excl overlap check by run()
	candidates.AndNot(exclude)    // exclude
	if candidates.Count() < n+1 { // +housekeeper
		return nil, fmt.Errorf("include/exclude are to strict")
	}
	if !include.IsSubset(candidates) {
		return nil, fmt.Errorf("include %s: offline or excluded cpu",
			include.String())
	}

	// evaluate
	scores, err := rankCPUs(topo, candidates, include)
	if err != nil {
		return nil, err
	}

	// selection
	var selection, cores CPUSet
	for _, s := range scores[:n] {
		selection.SetBit(s.CPU)
		cores.SetBit(topo.Core[s.CPU])
	}

	// housekeeper: first non previsouly selected
	housekeeper := scores[n].CPU
	for _, s := range scores[n:] {
		if !cores.GetBit(topo.Core[s.CPU]) {
			housekeeper = s.CPU
			break
		}
	}

	return &SelectionResult{
		Benchmark:   selection,
		HouseKeeper: housekeeper,
		Scores:      scores,
	}, nil
}
