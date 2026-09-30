package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/isolation"
)

func Isolation(cfg *cli.Config, topo *cpu.Topology, selected *generic.Selection) (*isolation.Cgroup, *isolation.Steering, func(*error), error) {
	var locks []*isolation.Lock
	var group *isolation.Cgroup
	var steering *isolation.Steering
	teardown := func() error {
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
		if group, err = isolation.InitCgroup(cgroupName, selected.Booked(), selected.Numa); err != nil {
			return nil, nil, nil, errors.Join(err, teardown())
		}
	}

	if cfg.Steering {
		lock, err := isolation.AcquireLock(filepath.Join(generic.RunLockDir, generic.SteerLockName))
		if err != nil {
			return nil, nil, nil, errors.Join(err, teardown())
		}
		locks = append(locks, lock)
		if steering, err = isolation.Steer(topo, selected.Booked(), selected.HouseKeeper); err != nil {
			return nil, nil, nil, errors.Join(err, teardown())
		}
	}

	release := func(runErr *error) {
		failed := teardown()
		if failed == nil {
			return
		}
		fmt.Fprintf(os.Stderr, "%steardown: %v\n", generic.LogPrefix, failed)
		if *runErr == nil {
			*runErr = errors.New("isolation teardown incomplete")
		}
	}
	return group, steering, release, nil
}

func releaseLocks(locks []*isolation.Lock) {
	for _, lock := range locks {
		lock.Release()
	}
}
