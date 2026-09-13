package cpu

import (
	"cmp"
	"slices"

	"goset/internal/telemetry"
)

type CPUScore struct {
	CPU          int
	Included     uint8  // 0|1
	Steerable    uint64 // numbered rows
	NonSteerable uint64 // named rows: LOC, RES, CAL, TLB
	SiblingLoad  uint64 // IRQs of the SMT siblings
	Node         int
	KernelIsol   bool
	NohzFull     bool
}


func rankCPUs(topo *Topology, candidates, include CPUSet) ([]CPUScore, error) {
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

	irqs, err := telemetry.ReadIRQCounts()
	if err != nil {
		return nil, err
	}

	irqOf := func(cpu int) telemetry.IRQCount {
		if cpu < len(irqs) {
			return irqs[cpu]
		}
		return telemetry.IRQCount{}
	}

	out := make([]CPUScore, 0, candidates.Count())
	for cand := range candidates.All() {
		var siblingLoad uint64
		for sib := range topo.Online.All() {
			if sib != cand && topo.Core[sib] == topo.Core[cand] {
				sibIRQ := irqOf(sib)
				siblingLoad += sibIRQ.Steerable + sibIRQ.NonSteerable
			}
		}
		irq := irqOf(cand)
		score := CPUScore{
			CPU:          cand,
			Steerable:    irq.Steerable,
			NonSteerable: irq.NonSteerable,
			SiblingLoad:  siblingLoad,
			Node:         topo.NumaNode[cand],
			KernelIsol:   topo.KernelIsol.GetBit(cand),
			NohzFull:     topo.NohzFull.GetBit(cand),
		}
		if include.GetBit(cand) {
			score.Included = 1
		}
		out = append(out, score)
	}

	// sort
	slices.SortStableFunc(out, func(a, b CPUScore) int { // IRQ
		return cmp.Compare(
			a.Steerable+a.NonSteerable+a.SiblingLoad,
			b.Steerable+b.NonSteerable+b.SiblingLoad)
	})
	slices.SortStableFunc(out, func(a, b CPUScore) int { // include
		return cmp.Compare(b.Included, a.Included) // a: false, //b:true
	})
	return out, nil
}
