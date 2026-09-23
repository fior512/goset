package runner

import (
	"path/filepath"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/isolation"
)

func startIsolation(cfg *cli.Config, topo *cpu.Topology, selected *generic.Selection) (*isolation.Cgroup, *isolation.Steering, func(), error) {
	var group *isolation.Cgroup
	if cfg.Cgroup {
		cgroupName := generic.CgroupIdentifier + filepath.Base(cfg.Task[0])
		var err error
		if group, err = isolation.InitCgroup(cgroupName, selected.Task, cfg.NumaNode); err != nil {
			return nil, nil, nil, err
		}
	}

	var steering *isolation.Steering
	if cfg.Steering {
		var err error
		if steering, err = isolation.Steer(topo, selected.Task, selected.HouseKeeper); err != nil {
			group.Destroy()
			return nil, nil, nil, err
		}
	}

	release := func() {
		group.Destroy() // cgroup
		if steering == nil {
			return
		}
		steering.RestoreIRQs() // Steer
		steering.Release() // IRQBalance
	}
	return group, steering, release, nil
}
