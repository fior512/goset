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

	selection, err := cpu.SelectCPUs(topo, n, cfg.Include, cfg.Exclude, cfg.PreferNode)
	if err != nil {
		return err
	}
	fmt.Printf("%d threads got selected \n", selection.Benchmark.Count())
	fmt.Printf("Which are: ")
	for cpu := range selection.Benchmark.All() {
		fmt.Printf("%d, ", cpu)
	}
	fmt.Printf("HouseKeeper is %d \n", selection.HouseKeeper)


	return isolation.ApplyPin(cfg.Cmd, selection.Benchmark)
}
