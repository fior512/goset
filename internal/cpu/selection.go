package cpu

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	"goset/internal/generic"
)

func SelectCPUs(topo *Topology, n int, include generic.CPUSet, exclude generic.CPUSet, numa int, fence bool) (*generic.Selection, error) {
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
	budget := topo.Online.Count()
	if fence {
		budget = bookingBudget(topo)
	}
	picks := PickCPUs(topo, scores, n, budget)
	var selection, cores generic.CPUSet
	for _, score := range picks {
		selection.SetBit(score.CPU)
		cores.SetBit(topo.Core[score.CPU])
	}
	housekeeper := pickHousekeeper(topo, scores, selection, cores)

	var siblings generic.CPUSet
	if fence {
		siblings = fenceSiblings(topo, cores, selection, exclude, housekeeper)
		trimFence(&siblings, scores, selection.Count(), bookingBudget(topo))
	}

	return &generic.Selection{
		Scores:      scores,
		Numa:        numa,
		Task:        selection,
		Fence:       siblings,
		HouseKeeper: housekeeper,
	}, nil
}

func fenceSiblings(topo *Topology, cores, task, exclude generic.CPUSet, housekeeper int) generic.CPUSet {
	var siblings generic.CPUSet
	for cpu := range topo.Online.All() {
		if cores.GetBit(topo.Core[cpu]) && !task.GetBit(cpu) && !exclude.GetBit(cpu) && cpu != housekeeper {
			siblings.SetBit(cpu)
		}
	}
	return siblings
}

func pickHousekeeper(topo *Topology, scores []generic.CPUScore, selection, cores generic.CPUSet) int {
	fallback := -1
	for _, score := range scores {
		if selection.GetBit(score.CPU) {
			continue
		}
		if !cores.GetBit(topo.Core[score.CPU]) {
			return score.CPU
		}
		if fallback < 0 {
			fallback = score.CPU
		}
	}
	return fallback
}

func bookingBudget(topo *Topology) int {
	online := topo.Online.Count()
	return online - max(generic.BookingReserveMin, online/generic.BookingReserveShare)
}

func coreSize(topo *Topology, cpu int) int {
	size := 0
	for other := range topo.Online.All() {
		if topo.Core[other] == topo.Core[cpu] {
			size++
		}
	}
	return size
}


func PickCPUs(topo *Topology, scores []generic.CPUScore, n, budget int) []generic.CPUScore {
	coreNoise := map[int]uint64{}
	for _, score := range scores {
		coreNoise[topo.Core[score.CPU]] += score.NonSteerable
	}

	var picked []generic.CPUScore
	var cores generic.CPUSet
	taken := map[int]bool{}
	booked := 0
	for len(picked) < n {
		forced := slices.ContainsFunc(scores, func(score generic.CPUScore) bool {
			return !taken[score.CPU] && score.Included == 1
		})
		best, bestFits := -1, false
		var bestCost, bestSteerable uint64
		for i, score := range scores {
			if taken[score.CPU] || (forced && score.Included == 0) {
				continue
			}
			fresh := !cores.GetBit(topo.Core[score.CPU])
			cost := score.NonSteerable
			if fresh {
				cost = score.NonSteerable + coreNoise[topo.Core[score.CPU]]
			}
			fits := !fresh || booked+coreSize(topo, score.CPU) <= budget
			cheaper := cmp.Or(cmp.Compare(cost, bestCost), cmp.Compare(score.Steerable, bestSteerable)) < 0
			better := best < 0 || (fits && !bestFits) || (fits == bestFits && cheaper)
			if better {
				best, bestFits, bestCost, bestSteerable = i, fits, cost, score.Steerable
			}
		}
		chosen := scores[best]
		if !cores.GetBit(topo.Core[chosen.CPU]) {
			booked += coreSize(topo, chosen.CPU)
			cores.SetBit(topo.Core[chosen.CPU])
		}
		taken[chosen.CPU] = true
		picked = append(picked, chosen)
	}
	return picked
}

func trimFence(fence *generic.CPUSet, scores []generic.CPUScore, task, budget int) {
	for task+fence.Count() > budget && fence.Any() {
		worst := -1
		var worstNoise uint64
		for cpu := range fence.All() {
			for _, score := range scores {
				if score.CPU == cpu && (worst < 0 || score.NonSteerable > worstNoise) {
					worst, worstNoise = cpu, score.NonSteerable
				}
			}
		}
		if worst < 0 {
			worst = fence.NextSet(0)
		}
		fence.ClearBit(worst)
	}
}
