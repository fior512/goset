package cpu

import (
	"fmt"
	"os"
	"strconv"

	"goset/internal/cli"
	"goset/internal/generic"
)

func Selection(topology *Topology, config *cli.Config) (*generic.Selection, error) {
	/* rank */
	candidates, numa, err := selectCandidates(topology, config)
	if err != nil {
		return nil, err
	}
	scores, err := rankCPUs(topology, candidates, config)
	if err != nil {
		return nil, err
	}

	/* select */
	var task generic.CPUSet
	for _, score := range scores[:config.NThreads] { // select CPUs for task
		task.SetBit(score.CPU)
	}
	housekeeper := selectHousekeeper(topology, scores, task)
	fence, unfenced := selectFence(topology, scores, task, housekeeper, config)
	if unfenced.Any() {
		fmt.Fprintf(os.Stderr, "%sfence: cpu %s left open, no room beside the housekeeper and irq steering, or excluded\n",
			generic.LogPrefix, unfenced.String())
	}
	return &generic.Selection{
		Scores:      scores,
		Numa:        numa,
		Task:        task,
		Fence:       fence,
		Unfenced:    unfenced,
		HouseKeeper: housekeeper,
	}, nil
}

func selectCandidates(topology *Topology, config *cli.Config) (generic.CPUSet, int, error) {
	candidates := topology.Online

	// incl&excl overlap check by run()
	candidates.AndNot(config.Exclude)
	if !config.Include.IsSubset(candidates) {
		return generic.CPUSet{}, 0, fmt.Errorf("include %s: offline or excluded cpu",
			config.Include.String())
	}

	/* numa */
	candidates, numa, err := constrainNuma(topology, candidates, config.Include, config.Numa)
	if err != nil {
		return generic.CPUSet{}, 0, err
	}

	/* minimum */
	scope := " on any numa node"
	if numa >= 0 {
		scope = " on numa " + strconv.Itoa(numa)
	}
	if candidates.Count() < config.NThreads+1 { // +housekeeper
		return generic.CPUSet{}, 0, fmt.Errorf(
			"need %d cpu(s) plus 1 housekeeper but only %d candidate(s) remain%s",
			config.NThreads, candidates.Count(), scope)
	}

	/* steer */
	if config.Steering && topology.Online.Count() < config.NThreads+2 { // +housekeeper +steered placeholder
		return generic.CPUSet{}, 0, fmt.Errorf(
			"-steer needs a cpu outside the %d task cpu(s) and the housekeeper, only %d online",
			config.NThreads, topology.Online.Count())
	}
	return candidates, numa, nil
}

func selectHousekeeper(topology *Topology, scores []generic.CPUScore, task generic.CPUSet) int {
	/* cores */
	var taskCores generic.CPUSet
	for cpu := range task.All() {
		taskCores.SetBit(topology.Core[cpu])
	}

	/* search */
	fallback := -1
	for _, score := range scores {
		if task.GetBit(score.CPU) {
			continue
		}
		if !taskCores.GetBit(topology.Core[score.CPU]) {
			return score.CPU
		}
		if fallback < 0 {
			fallback = score.CPU
		}
	}
	return fallback
}

func selectFence(topology *Topology, scores []generic.CPUScore, task generic.CPUSet, housekeeper int, config *cli.Config) (fence, unfenced generic.CPUSet) {
	if !config.Fence {
		return fence, unfenced
	}

	/* room */
	room := topology.Online.Count() - task.Count() - 1 // -housekeeper
	if config.Steering {
		room-- // steered place
	}

	/* siblings */
	var taskCores, siblings generic.CPUSet
	for cpu := range task.All() {
		taskCores.SetBit(topology.Core[cpu])
	}
	for cpu := range topology.Online.All() {
		if taskCores.GetBit(topology.Core[cpu]) && !task.GetBit(cpu) && cpu != housekeeper {
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
