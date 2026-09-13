package runner

import (
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

	return isolation.ApplyPin(cfg.Cmd, selection.Benchmark)
}
