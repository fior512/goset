package runner

import (
	"path/filepath"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/isolation"
)

func startIsolation(cfg *cli.Config, topo *cpu.Topology, selected *generic.Selection) (*isolation.Cgroup, *isolation.Steering, func(), error) {
	var locks []*isolation.Lock
	var group *isolation.Cgroup
	if cfg.Cgroup {
		cgroupName := generic.CgroupIdentifier + filepath.Base(cfg.Task[0])
		lock, err := isolation.AcquireLock(filepath.Join(generic.RunLockDir, cgroupName+".lock"))
		if err != nil {
			return nil, nil, nil, err
		}
		locks = append(locks, lock)
		if group, err = isolation.InitCgroup(cgroupName, selected.Task, selected.Numa); err != nil {
			releaseLocks(locks)
			return nil, nil, nil, err
		}
	}

	var steering *isolation.Steering
	if cfg.Steering {
		lock, err := isolation.AcquireLock(filepath.Join(generic.RunLockDir, generic.SteerLockName))
		if err != nil {
			group.Destroy()
			releaseLocks(locks)
			return nil, nil, nil, err
		}
		locks = append(locks, lock)
		if steering, err = isolation.Steer(topo, selected.Task, selected.HouseKeeper); err != nil {
			group.Destroy()
			releaseLocks(locks)
			return nil, nil, nil, err
		}
	}

	release := func() {
		group.Destroy() // cgroup
		if steering != nil {
			steering.RestoreIRQs() // Steer
			steering.Release()     // IRQBalance
		}
		releaseLocks(locks)
	}
	return group, steering, release, nil
}

func releaseLocks(locks []*isolation.Lock) {
	for _, lock := range locks {
		lock.Release()
	}
}
