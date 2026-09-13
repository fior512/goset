package cpu

import (
	"fmt"
	"sort"
)

// CPUScore score per Threads
type CPUScore struct {
	CPU          int
	Steerable    uint64
	NonSteerable uint64
	Total        uint64
	SiblingLoad  uint64
	Node         int
	KernalIsol   bool
	NohzFull     bool
}


// SelectionResult Benchmark Results
type SelectionResult struct {
	Benchmark   CPUSet
	HouseKeeper int
	Scores      []CPUScore
	SamplingMS  int
	Node        int
}

func bench(candidates CPUSet, include CPUSet) ([]CPUScore, error) {
	var out []CPUScore // 0..incl..candidates

	/*
	include{1,2,5} // threads id requested
	candidates{1,2,3,4,5,6} // all available threads

	out (id, include?, IRQ){
		{2, 1, 3},
		{5, 1, 3},
		{1, 1, 5},
		{3, 0, 2},
		{4, 0, 3},
		{6, 0, 10}
	}

	in order to have no if, just one loop (handles all incl sizes: incl > N && incl == N && incl < N )
	for id := range N { } // done in SelectCPUs

	//Rules:
	//  1) Include on top, and create a "cluster"
	//  2) sort out while maintaining the include "cluster"
	//				so we pick include while still lowering IRQ (and other topo aspects)
	*/
	
	return out
}

func SelectCPUs(topo *Topology, n int, include CPUSet, exclude CPUSet, samplingMS int, preferNode bool) (*SelectionResult, error) {
	candidates := topo.Online // threads available

	// incl&excl overlap check by run()
	candidates.AndNot(exclude) // exclude
	if candidates.Count() >= n+1 { // +housekeeper
		return nil, fmt.Errorf("include/exclude are to strict")
	}
	if !include.IsSubset(candidates) { 
		//
	}




}
