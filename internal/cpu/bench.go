package cpu

import (
	"cmp"
	"slices"
	"time"

	"goset/internal/cli"
	"goset/internal/generic"

	"goset/internal/telemetry"
)

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

func cpuNoise(irq telemetry.IRQCount, steer bool) uint64 {
	if steer {
		return irq.NonSteerable
	}
	return irq.NonSteerable + irq.Steerable
}

func siblingLoads(topo *Topology, delta []telemetry.IRQCount, steer bool) map[int]uint64 {
	coreTotal := make(map[int]uint64, len(topo.Core))
	for cpu := range topo.Online.All() {
		coreTotal[topo.Core[cpu]] += cpuNoise(deltaAt(delta, cpu), steer)
	}

	sibling := make(map[int]uint64, len(topo.Core))
	for cpu := range topo.Online.All() {
		sibling[cpu] = coreTotal[topo.Core[cpu]] - cpuNoise(deltaAt(delta, cpu), steer)
	}
	return sibling
}

// maps each thread to the other thread of its core, -1 when it has none
func siblingCPUs(topo *Topology) map[int]int {
	pending := map[int]int{}
	out := make(map[int]int, len(topo.Core))
	for cpu := range topo.Online.All() {
		core := topo.Core[cpu]
		other, seen := pending[core]
		if !seen {
			pending[core] = cpu
			out[cpu] = -1
			continue
		}
		delete(pending, core)
		out[cpu] = other
		out[other] = cpu
	}
	return out
}

func rankCPUs(topo *Topology, candidates generic.CPUSet, cfg *cli.Config) ([]generic.CPUScore, error) {
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

	// sample
	delta, err := sampleIRQDelta(500 * time.Millisecond)
	if err != nil {
		return nil, err
	}
	sibling := siblingLoads(topo, delta, cfg.Steering)
	threads := siblingCPUs(topo)

	// saving
	out := make([]generic.CPUScore, 0, candidates.Count())
	for cpu := range candidates.All() {
		irq := deltaAt(delta, cpu)
		score := generic.CPUScore{
			CPU:          cpu,
			Steerable:    irq.Steerable,
			NonSteerable: irq.NonSteerable,
			Noise:        cpuNoise(irq, cfg.Steering),
			SiblingLoad:  sibling[cpu],
			Sibling:      threads[cpu],
			Numa:         topo.Numa[cpu],
			KernelIsol:   topo.KernelIsol.GetBit(cpu),
			NohzFull:     topo.NohzFull.GetBit(cpu),
			RcuNocb:      topo.RcuNocb.GetBit(cpu),
		}
		if cfg.Include.GetBit(cpu) {
			score.Included = 1
		}
		out = append(out, score)
	}

	/* rank */
	slices.SortStableFunc(out, func(left, right generic.CPUScore) int {
		return cmp.Or(
			cmp.Compare(right.Included, left.Included), // desc
			cmp.Compare(left.Noise+left.SiblingLoad, right.Noise+right.SiblingLoad),
			cmp.Compare(left.Noise, right.Noise),
			cmp.Compare(left.Steerable, right.Steerable),
			cmp.Compare(topo.Core[left.CPU], topo.Core[right.CPU]),
			cmp.Compare(left.CPU, right.CPU),
		)
	})
	return out, nil
}
