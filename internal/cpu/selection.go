package cpu

import (
	"fmt"
	"strconv"

	"goset/internal/generic"
)

// SelectCPUs plans a CPU booking
func SelectCPUs(topo *Topology, request generic.SelectionRequest) (*generic.Selection, error) {
	/* rank */
	candidates, numa, err := selectCandidates(topo, request)
	if err != nil {
		return nil, err
	}
	scores, err := rankCPUs(topo, candidates, request)
	if err != nil {
		return nil, err
	}

	/* select */
	var task generic.CPUSet
	for _, score := range scores[:request.N] { // select CPUs for task
		task.SetBit(score.CPU)
	}
	housekeeper := selectHousekeeper(topo, scores, task)
	fence, unfenced := selectFence(topo, scores, task, housekeeper, request)
	return &generic.Selection{
		Scores:      scores,
		Numa:        numa,
		Task:        task,
		Fence:       fence,
		Unfenced:    unfenced,
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

	/* steer */
	if request.Steer && topo.Online.Count() < request.N+2 { // +housekeeper +steered placeholder
		return generic.CPUSet{}, 0, fmt.Errorf(
			"-steer needs a cpu outside the %d task cpu(s) and the housekeeper, only %d online",
			request.N, topo.Online.Count())
	}
	return candidates, numa, nil
}

func selectHousekeeper(topo *Topology, scores []generic.CPUScore, task generic.CPUSet) int {
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

func selectFence(topo *Topology, scores []generic.CPUScore, task generic.CPUSet, housekeeper int, request generic.SelectionRequest) (fence, unfenced generic.CPUSet) {
	if !request.Fence {
		return fence, unfenced
	}

	/* room */
	room := topo.Online.Count() - task.Count() - 1 // -housekeeper
	if request.Steer {
		room-- // steered place
	}

	/* siblings */
	var taskCores, siblings generic.CPUSet
	for cpu := range task.All() {
		taskCores.SetBit(topo.Core[cpu])
	}
	for cpu := range topo.Online.All() {
		if taskCores.GetBit(topo.Core[cpu]) && !task.GetBit(cpu) && cpu != housekeeper {
			siblings.SetBit(cpu)
		}
	}

	/* fill */
	// scores are ranked: the quietest siblings fill the room first
	for _, score := range scores {
		if siblings.GetBit(score.CPU) && fence.Count() < room {
			fence.SetBit(score.CPU)
		}
	}
	unfenced = siblings
	unfenced.AndNot(fence)
	return fence, unfenced
}
