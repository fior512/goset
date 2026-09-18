package cpu

import (
	"fmt"
)

// SelectionResult Benchmark Results
type SelectionResult struct {
	Scores      []CPUScore
	Benchmark   CPUSet
	HouseKeeper int
}


func SelectCPUs(topo *Topology, n int, include CPUSet, exclude CPUSet, NumaNode int) (*SelectionResult, error) {
	candidates := topo.Online

	watch := n
	if watch == 0 {
		watch = 1
	}

	// incl&excl overlap check by run()
	candidates.AndNot(exclude)        // exclude
	if candidates.Count() < watch+1 { // +housekeeper
		return nil, fmt.Errorf(
			"need %d cpu(s) plus 1 housekeeper but only %d candidate(s) remain",
			watch, candidates.Count())
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
	for _, score := range scores[:watch] {
		selection.SetBit(score.CPU)
		cores.SetBit(topo.Core[score.CPU])
	}

	// housekeeper: first non previsouly selected
	housekeeper := scores[watch].CPU
	for _, score := range scores[watch:] {
		if !cores.GetBit(topo.Core[score.CPU]) {
			housekeeper = score.CPU
			break
		}
	}

	return &SelectionResult{
		Scores:      scores,
		Benchmark:   selection,
		HouseKeeper: housekeeper,
	}, nil
}
