package cpu

import (
	"slices"

	"goset/internal/generic"
)

type booking struct {
	topo      *Topology
	scores    []generic.CPUScore
	budget    int
	coreNoise map[int]uint64 // core -> non-steerable IRQs summed over its candidate cpus
	coreSize  map[int]int    // core -> online cpus
	task      generic.CPUSet
	cores     generic.CPUSet // cores holding a task cpu
	booked    int
}

func (plan *booking) nextCPU() (generic.CPUScore, bool) {
	/* forced */
	forced := slices.ContainsFunc(plan.scores, func(score generic.CPUScore) bool {
		return !plan.task.GetBit(score.CPU) && score.Included == 1
	})

	/* scan */
	var best generic.CPUScore
	var bestCost uint64
	var found, bestFits bool
	for _, score := range plan.scores {
		if plan.task.GetBit(score.CPU) || (forced && score.Included == 0) {
			continue
		}
		core := plan.topo.Core[score.CPU]
		cost, fits := score.NonSteerable, true
		if !plan.cores.GetBit(core) {
			cost += plan.coreNoise[core]
			fits = plan.booked+plan.coreSize[core] <= plan.budget
		}
		tie := cost == bestCost && score.Steerable < best.Steerable
		if !found || (fits && !bestFits) || (fits == bestFits && (cost < bestCost || tie)) {
			best, bestCost, bestFits, found = score, cost, fits, true
		}
	}
	return best, found
}

func (plan *booking) add(score generic.CPUScore) {
	core := plan.topo.Core[score.CPU]
	if !plan.cores.GetBit(core) {
		plan.cores.SetBit(core)
		plan.booked += plan.coreSize[core]
	}
	plan.task.SetBit(score.CPU)
}
