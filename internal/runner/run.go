package runner

import (
	"fmt"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/isolation"
)

func Run(cfg *cli.Config) error {
	topo, err := cpu.GetTopology()
	if err != nil {
		return err
	}

	n := max(cfg.Cgroup, 1)

	selection, err := cpu.SelectCPUs(topo, n, cfg.Include, cfg.Exclude, cfg.NumaNode)
	if err != nil {
		return err
	}

	//TODO: Replace with telemetry and report sys
	fmt.Printf("%d threads got selected \n", selection.Benchmark.Count())
	fmt.Printf("Which are: ")
	for cpu := range selection.Benchmark.All() {
		fmt.Printf("%d, ", cpu)
	}
	fmt.Printf("HouseKeeper is %d \n", selection.HouseKeeper)


	
	var group *isolation.Cgroup
	if n > 1 { //TODO: ducktape check
		var err error	
		group, err = isolation.InitCgroup(cfg.Task[0], selection.Benchmark, -1) //TODO: handle NumaNode
		if err != nil {
			return err
		}
	}
	return isolation.ApplyPin(cfg.Task, selection.Benchmark, group)
}
