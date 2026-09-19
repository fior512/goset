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
	NumaNode         int
	KernelIsol   bool
	NohzFull     bool
	RcuNocb      bool
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


func deltaAt(delta []telemetry.IRQCount, cpu int) telemetry.IRQCount {
	if cpu < 0 || cpu >= len(delta) {
		return telemetry.IRQCount{}
	}
	return delta[cpu]
}


func siblingLoads(topo *Topology, delta []telemetry.IRQCount) map[int]uint64 {
	coreTotal := make(map[int]uint64, len(topo.Core))
	for cpu := range topo.Online.All() {
		irq := deltaAt(delta, cpu)
		coreTotal[topo.Core[cpu]] += irq.Steerable + irq.NonSteerable
	}

	sibling := make(map[int]uint64, len(topo.Core))
	for cpu := range topo.Online.All() {
		own := deltaAt(delta, cpu)
		sibling[cpu] = coreTotal[topo.Core[cpu]] - (own.Steerable + own.NonSteerable)
	}
	return sibling
}


func rankCPUs(topo *Topology, candidates, include CPUSet, NumaNode int) ([]CPUScore, error) {
	/*
		include{1,2,5} // threads id requested
		candidates{1,2,3,4,5,6} // all available threads

		out (id, include?, hard IRQ){
			{2, 1, 3},
			{5, 1, 3},
			{1, 1, 5},
			{3, 0, 2},
			{4, 0, 3},
			{6, 0, 10}
		}
		Then pick the top N element.
		It imply include[] size invariance logic



		//Rules:
		//  1) Include on top, and create a "cluster"
		//  2) sort out while maintaining the include "cluster"
		//				so we pick include while still lowering IRQ
	*/

	delta, err := sampleIRQDelta(500 * time.Millisecond)
	if err != nil {
		return nil, err
	}
	sibling := siblingLoads(topo, delta)

	// saving
	out := make([]CPUScore, 0, candidates.Count())
	for cpu := range candidates.All() {
		irq := deltaAt(delta, cpu)
		score := CPUScore{
			CPU:          cpu,
			Steerable:    irq.Steerable,
			NonSteerable: irq.NonSteerable,
			SiblingLoad:  sibling[cpu],
			NumaNode:         topo.NumaNode[cpu],
			KernelIsol:   topo.KernelIsol.GetBit(cpu),
			NohzFull:     topo.NohzFull.GetBit(cpu),
			RcuNocb:      topo.RcuNocb.GetBit(cpu),
		}
		if include.GetBit(cpu) {
			score.Included = 1
		}
		out = append(out, score)
	}

	// NumaNode
	var node int
	if NumaNode == -1 && len(out) > 0 {
		// AUTO Numa
		node = topo.NumaNode[out[0].CPU] // top
	}

	// TODO: find better bool sort
	b2i := func(condition bool) int {
		if NumaNode == -2 { return 0 } // OFF
		if condition { return 1 }
		return -1
	}

	// sorting
	slices.SortStableFunc(out, func(left, right CPUScore) int {
		return cmp.Or(
			cmp.Compare(right.Included, left.Included),                   // desc
			cmp.Compare(b2i(right.NumaNode == node), b2i(left.NumaNode == node)), // desc
			cmp.Compare(left.NonSteerable, right.NonSteerable),
			cmp.Compare(left.SiblingLoad, right.SiblingLoad),
			cmp.Compare(left.Steerable, right.Steerable),
		)
	})
	return out, nil
}
