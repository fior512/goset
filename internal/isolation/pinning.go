package isolation

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"goset/internal/cpu"
)

func ApplyPin(argv []string, cpus cpu.CPUSet, group *Cgroup) error {
	task := exec.Command(argv[0], argv[1:]...)
	task.Stdin = os.Stdin
	task.Stdout = os.Stdout
	task.Stderr = os.Stderr
	task.SysProcAttr = &syscall.SysProcAttr{}

	if group != nil {
		task.SysProcAttr.UseCgroupFD = true
		task.SysProcAttr.CgroupFD = group.FD()
	}

	type result struct {
		err error
	}
	done := make(chan result, 1)
	go func() {
		runtime.LockOSThread()
		defer group.Destroy()
		defer runtime.UnlockOSThread()

		var prevCPUs cpu.CPUSet
		var gerr error
		if group == nil {
			prevCPUs, gerr = cpu.GetAffinity(0) // goset affinity
			if err := cpu.SetAffinity(0, cpus); err != nil {
				done <- result{fmt.Errorf("set thread affinity: %w", err)}
				return
			}
		}
		err := task.Run() // start and wait for the task to exit

		if group == nil && gerr == nil {
			_ = cpu.SetAffinity(0, prevCPUs)
		}
		done <- result{err}
	}()

	if res := <-done; res.err != nil {
		return res.err
	}

	return nil
}
