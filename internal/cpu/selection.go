package cpu

import (
	"fmt"
	"strconv"

	"goset/internal/generic"
)

func SelectCPUs(topo *Topology, request generic.SelectionRequest) (*generic.Selection, error) {
	/* rank */
	candidates, numa, err := selectCandidates(topo, request)
	if err != nil {
		return nil, err
	}
	scores, err := rankCPUs(topo, candidates, request.Include)
	if err != nil {
		return nil, err
	}

	/* select */
	task := SelectTask(topo, scores, request)
	housekeeper := SelectHousekeeper(topo, scores, task)
	return &generic.Selection{
		Scores:      scores,
		Numa:        numa,
		Task:        task,
		Fence:       SelectFence(topo, scores, task, housekeeper, request),
		HouseKeeper: housekeeper,
	}, nil
}

func selectCandidates(topo *Topology, request generic.SelectionRequest) (generic.CPUSet, int, error) {
	candidates := topo.Online

	// incl&excl overlap check by run()
	candidates.AndNot(request.Exclude)
	if !request.Include.IsSubset(candidates) {
		return generic.CPUSet{}, 0, fmt.Errorf("include %s: offline or excluded cpu",
			request.Include.String())
	}

	/* numa */
	candidates, numa, err := constrainNuma(topo, candidates, request.Include, request.Numa)
	if err != nil {
		return generic.CPUSet{}, 0, err
	}

	/* minimum */
	scope := " on any numa node"
	if numa >= 0 {
		scope = " on numa " + strconv.Itoa(numa)
	}
	if candidates.Count() < request.N+1 { // +housekeeper
		return generic.CPUSet{}, 0, fmt.Errorf(
			"need %d cpu(s) plus 1 housekeeper but only %d candidate(s) remain%s",
			request.N, candidates.Count(), scope)
	}
	return candidates, numa, nil
}

func SelectTask(topo *Topology, scores []generic.CPUScore, request generic.SelectionRequest) generic.CPUSet {
	/* budget */
	online := topo.Online.Count()
	budget := online
	if request.Fence {
		budget -= max(generic.BookingReserveMin, online/generic.BookingReserveShare)
	}

	/* cores */
	coreNoise := map[int]uint64{}
	for _, score := range scores {
		coreNoise[topo.Core[score.CPU]] += score.NonSteerable
	}
	coreSize := map[int]int{}
	for cpu := range topo.Online.All() {
		coreSize[topo.Core[cpu]]++
	}

	/* book */
	plan := &booking{topo: topo, scores: scores, budget: budget, coreNoise: coreNoise, coreSize: coreSize}
	for plan.task.Count() < request.N {
		score, found := plan.nextCPU()
		if !found {
			break
		}
		plan.add(score)
	}
	return plan.task
}

func SelectHousekeeper(topo *Topology, scores []generic.CPUScore, task generic.CPUSet) int {
	/* cores */
	var taskCores generic.CPUSet
	for cpu := range task.All() {
		taskCores.SetBit(topo.Core[cpu])
	}

	/* search */
	fallback := -1
	for _, score := range scores {
		if task.GetBit(score.CPU) {
			continue
		}
		if !taskCores.GetBit(topo.Core[score.CPU]) {
			return score.CPU
		}
		if fallback < 0 {
			fallback = score.CPU
		}
	}
	return fallback
}

func SelectFence(topo *Topology, scores []generic.CPUScore, task generic.CPUSet, housekeeper int, request generic.SelectionRequest) generic.CPUSet {
	var fence generic.CPUSet
	if !request.Fence {
		return fence
	}
	/* budget */
	online := topo.Online.Count()
	budget := online - max(generic.BookingReserveMin, online/generic.BookingReserveShare)

	/* cores */
	var taskCores generic.CPUSet
	for cpu := range task.All() {
		taskCores.SetBit(topo.Core[cpu])
	}

	/* fill */
	// scores are ranked: the quietest siblings fill the budget first
	for _, score := range scores {
		sibling := taskCores.GetBit(topo.Core[score.CPU]) && !task.GetBit(score.CPU) && score.CPU != housekeeper
		if sibling && task.Count()+fence.Count() < budget {
			fence.SetBit(score.CPU)
		}
	}
	return fence
}
