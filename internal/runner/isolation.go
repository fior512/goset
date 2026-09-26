package runner

import (
	"errors"
	"fmt"
	"path/filepath"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/isolation"
)

func startIsolation(cfg *cli.Config, topo *cpu.Topology, selected *generic.Selection) (*isolation.Cgroup, *isolation.Steering, func() error, error) {
	var locks []*isolation.Lock
	var group *isolation.Cgroup
	var steering *isolation.Steering
	release := func() error {
		var failures []error
		if err := group.Destroy(); err != nil {
			failures = append(failures, err)
		}
		restored, failed := steering.RestoreIRQs()
		if failed > 0 {
			failures = append(failures, fmt.Errorf(
				"restore smp_affinity_list: %d of %d failed", failed, restored+failed))
		}
		steering.Release()
		releaseLocks(locks)
		return errors.Join(failures...)
	}
	if cfg.Cgroup {
		cgroupName := generic.CgroupIdentifier + filepath.Base(cfg.Task[0])
		lock, err := isolation.AcquireLock(filepath.Join(generic.RunLockDir, cgroupName+".lock"))
		if err != nil {
			return nil, nil, nil, err
		}
		locks = append(locks, lock)
		if group, err = isolation.InitCgroup(cgroupName, selected.Task, selected.NumaNode); err != nil {
			return nil, nil, nil, errors.Join(err, release())
		}
	}

	if cfg.Steering {
		lock, err := isolation.AcquireLock(filepath.Join(generic.RunLockDir, generic.SteerLockName))
		if err != nil {
			return nil, nil, nil, errors.Join(err, release())
		}
		locks = append(locks, lock)
		if steering, err = isolation.Steer(topo, selected.Task, selected.HouseKeeper); err != nil {
			return nil, nil, nil, errors.Join(err, release())
		}
	}

	return group, steering, release, nil
}

func releaseLocks(locks []*isolation.Lock) {
	for _, lock := range locks {
		lock.Release()
	}
}
