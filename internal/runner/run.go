package runner

import (
	"os"
	"time"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/isolation"
	"goset/internal/telemetry"
)

func Run(cfg *cli.Config) error {
	topo, err := cpu.GetTopology()
	if err != nil {
		return err
	}
	n := max(cfg.Cgroup, 1)

	// Selection
	selection, err := cpu.SelectCPUs(topo, n, cfg.Include, cfg.Exclude, cfg.NumaNode)
	if err != nil {
		return err
	}

	// Cgroup
	// TODO: define if cgroup with single thread is worth
	var group *isolation.Cgroup
	if n > 1 { // TODO: ducktape check
		var err error
		group, err = isolation.InitCgroup(cfg.Task[0], selection.Benchmark, -1) // TODO: handle NumaNode
		if err != nil {
			return err
		}
	}

	// Telemetry
	//TODO: InitCgroup and ApplyPin use selection._thing_
	//TODO: choose one api
	sampler, err := startTelemetry(selection, cfg) 
	if err != nil {
		return err
	}

	started := time.Now()
	runErr := isolation.ApplyPin(cfg.Task, selection.Benchmark, group) //TODO: extract Task.start()
	wall := time.Since(started)

	metrics := sampler.Stop()
	telemetry.Print(os.Stdout, telemetry.NewReport(wall, metrics))
	return runErr
}
