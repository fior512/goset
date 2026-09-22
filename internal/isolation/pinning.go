package isolation

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"goset/internal/cpu"
	"goset/internal/generic"
)


func ApplyPn(argv []string, cpus generic.CPUSet, group *Cgroup) (*exec.Cmd, error) {
	// Task In/Out
	task := exec.Command(argv[0], argv[1:]...)
	task.Stdin = os.Stdin
	task.Stdout = os.Stdout
	task.Stderr = os.Stderr
	task.SysProcAttr = &syscall.SysProcAttr{}

	if group != nil {
		task.SysProcAttr.UseCgroupFD = true
		task.SysProcAttr.CgroupFD = group.FD()
	}

	// Run Protocol
	started := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if group != nil {
			started <- task.Start()
			return
		}

		prevCPUs, gerr := cpu.GetAffinity(0) // goset affinity
		if err := cpu.SetAffinity(0, cpus); err != nil {
			started <- fmt.Errorf("set thread affinity: %w", err)
			return
		}
		err := task.Start()
		if gerr == nil {
			_ = cpu.SetAffinity(0, prevCPUs)
		}
		started <- err
	}()

	if err := <-started; err != nil {
		return nil, err
	}
	return task, nil
}
