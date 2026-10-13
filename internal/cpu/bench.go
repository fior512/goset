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
	before, err := telemetry.ReadIRQCounts(generic.ProcRoot)
	if err != nil {
		return nil, err
	}
	time.Sleep(interval)
	after, err := telemetry.ReadIRQCounts(generic.ProcRoot)
	if err != nil {
		return nil, err
	}

	delta := make([]telemetry.IRQCount, len(after))
	for i := range after {
		var irqcount telemetry.IRQCount
		if i < len(before) {
			irqcount = before[i]
		}
		delta[i] = telemetry.IRQCount{
			Steerable:    after[i].Steerable - irqcount.Steerable,
			NonSteerable: after[i].NonSteerable - irqcount.NonSteerable,
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

func siblingLoads(topology *Topology, delta []telemetry.IRQCount, steer bool) map[int]uint64 {
	coreTotal := make(map[int]uint64, len(topology.Core))
	for cpu := range topology.Online.All() {
		coreTotal[topology.Core[cpu]] += cpuNoise(deltaAt(delta, cpu), steer)
	}

	sibling := make(map[int]uint64, len(topology.Core))
	for cpu := range topology.Online.All() {
		sibling[cpu] = coreTotal[topology.Core[cpu]] - cpuNoise(deltaAt(delta, cpu), steer)
	}
	return sibling
}

// maps each thread to the other thread of its core, -1 when it has none
func siblingCPUs(topology *Topology) map[int]int {
	pending := map[int]int{}
	out := make(map[int]int, len(topology.Core))
	for cpu := range topology.Online.All() {
		core := topology.Core[cpu]
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

func rankCPUs(topology *Topology, candidates generic.CPUSet, config *cli.Config) ([]generic.CPUScore, error) {
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
	sibling := siblingLoads(topology, delta, config.Steering)
	threads := siblingCPUs(topology)

	// saving
	out := make([]generic.CPUScore, 0, candidates.Count())
	for cpu := range candidates.All() {
		irq := deltaAt(delta, cpu)
		score := generic.CPUScore{
			CPU:          cpu,
			Steerable:    irq.Steerable,
			NonSteerable: irq.NonSteerable,
			Noise:        cpuNoise(irq, config.Steering),
			SiblingLoad:  sibling[cpu],
			Sibling:      threads[cpu],
			Numa:         topology.Numa[cpu],
			KernelIsol:   topology.KernelIsol.GetBit(cpu),
			NohzFull:     topology.NohzFull.GetBit(cpu),
			RcuNocb:      topology.RcuNocb.GetBit(cpu),
		}
		if config.Include.GetBit(cpu) {
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
			cmp.Compare(topology.Core[left.CPU], topology.Core[right.CPU]),
			cmp.Compare(left.CPU, right.CPU),
		)
	})
	return out, nil
}
