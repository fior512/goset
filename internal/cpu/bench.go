package cpu

import (
	"cmp"
	"slices"
	"time"

	"goset/internal/telemetry"
)

type CPUScore struct {
	CPU          int
	Included     uint8  // 0|1
	Steerable    uint64 // numbered rows
	NonSteerable uint64 // named rows: LOC, RES, CAL, TLB
	SiblingLoad  uint64 // IRQs of the SMT siblings, self excluded
	Node         int    // numa
	KernelIsol   bool
	NohzFull     bool
	RcuNocb      bool
}

//TODO: find a sweet place to hold it
func Ternary[T any](condition bool, trueVal, falseVal T) T {
	if condition {
		return trueVal
	}
	return falseVal
}


func sampleIRQDelta(interval time.Duration) ([]telemetry.IRQCount, error) {
	before, err := telemetry.ReadIRQCounts()
	if err != nil {
		return nil, err
	}
	time.Sleep(interval)
	after, err := telemetry.ReadIRQCounts()
	if err != nil {
		return nil, err
	}

	delta := make([]telemetry.IRQCount, len(after))
	for i := range after {
		var b telemetry.IRQCount
		if i < len(before) {
			b = before[i]
		}
		delta[i] = telemetry.IRQCount{
			Steerable:    after[i].Steerable - b.Steerable,
			NonSteerable: after[i].NonSteerable - b.NonSteerable,
		}
	}
	return delta, nil
}


func siblingLoads(topo *Topology, delta []telemetry.IRQCount) map[int]uint64 {
	coreTotal := make(map[int]uint64, len(topo.Core))
	for cpu := range topo.Online.All() {
		irq := Ternary(cpu < len(delta), delta[cpu], telemetry.IRQCount{})
		coreTotal[topo.Core[cpu]] += irq.Steerable + irq.NonSteerable
	}

	sibling := make(map[int]uint64, len(topo.Core))
	for cpu := range topo.Online.All() {
		own := Ternary(cpu < len(delta), delta[cpu], telemetry.IRQCount{})
		sibling[cpu] = coreTotal[topo.Core[cpu]] - (own.Steerable + own.NonSteerable)
	}
	return sibling
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

	delta, err := sampleIRQDelta(100 * time.Millisecond)
	if err != nil {
		return nil, err
	}
	sibling := siblingLoads(topo, delta)

	//saving
	out := make([]CPUScore, 0, candidates.Count())
	for cpu := range candidates.All() {
		irq := Ternary(cpu < len(delta), delta[cpu], telemetry.IRQCount{})
		score := CPUScore{
			CPU:          cpu,
			Steerable:    irq.Steerable,
			NonSteerable: irq.NonSteerable,
			SiblingLoad:  sibling[cpu],
			Node:         topo.NumaNode[cpu],
			KernelIsol:   topo.KernelIsol.GetBit(cpu),
			NohzFull:     topo.NohzFull.GetBit(cpu),
			RcuNocb:      topo.RcuNocb.GetBit(cpu),
		}
		if include.GetBit(cpu) {
			score.Included = 1
		}
		out = append(out, score)
	}

	//sorting
	slices.SortStableFunc(out, func(a, b CPUScore) int {
		return cmp.Or(
			cmp.Compare(b.Included, a.Included), // desc
			cmp.Compare(a.NonSteerable, b.NonSteerable),
			cmp.Compare(a.SiblingLoad, b.SiblingLoad),
			cmp.Compare(a.Steerable, b.Steerable),
		)
	})
	return out, nil
}
