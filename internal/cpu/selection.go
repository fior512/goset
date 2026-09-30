package cpu

import (
	"fmt"
	"os"
	"strconv"

	"goset/internal/cli"
	"goset/internal/generic"
)

func Selection(topo *Topology, cfg *cli.Config) (*generic.Selection, error) {
	/* rank */
	candidates, numa, err := selectCandidates(topo, cfg)
	if err != nil {
		return nil, err
	}
	scores, err := rankCPUs(topo, candidates, cfg)
	if err != nil {
		return nil, err
	}

	/* select */
	var task generic.CPUSet
	for _, score := range scores[:cfg.NThreads] { // select CPUs for task
		task.SetBit(score.CPU)
	}
	housekeeper := selectHousekeeper(topo, scores, task)
	fence, unfenced := selectFence(topo, scores, task, housekeeper, cfg)
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

func selectCandidates(topo *Topology, cfg *cli.Config) (generic.CPUSet, int, error) {
	candidates := topo.Online

	// incl&excl overlap check by run()
	candidates.AndNot(cfg.Exclude)
	if !cfg.Include.IsSubset(candidates) {
		return generic.CPUSet{}, 0, fmt.Errorf("include %s: offline or excluded cpu",
			cfg.Include.String())
	}

	/* numa */
	candidates, numa, err := constrainNuma(topo, candidates, cfg.Include, cfg.Numa)
	if err != nil {
		return generic.CPUSet{}, 0, err
	}

	/* minimum */
	scope := " on any numa node"
	if numa >= 0 {
		scope = " on numa " + strconv.Itoa(numa)
	}
	if candidates.Count() < cfg.NThreads+1 { // +housekeeper
		return generic.CPUSet{}, 0, fmt.Errorf(
			"need %d cpu(s) plus 1 housekeeper but only %d candidate(s) remain%s",
			cfg.NThreads, candidates.Count(), scope)
	}

	/* steer */
	if cfg.Steering && topo.Online.Count() < cfg.NThreads+2 { // +housekeeper +steered placeholder
		return generic.CPUSet{}, 0, fmt.Errorf(
			"-steer needs a cpu outside the %d task cpu(s) and the housekeeper, only %d online",
			cfg.NThreads, topo.Online.Count())
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

func selectFence(topo *Topology, scores []generic.CPUScore, task generic.CPUSet, housekeeper int, cfg *cli.Config) (fence, unfenced generic.CPUSet) {
	if !cfg.Fence {
		return fence, unfenced
	}

	/* room */
	room := topo.Online.Count() - task.Count() - 1 // -housekeeper
	if cfg.Steering {
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
