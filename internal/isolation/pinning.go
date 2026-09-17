package isolation

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"goset/internal/cpu"
)

func ApplyPin(argv []string, cpus cpu.CPUSet, group *Cgroup) (*syscall.Rusage, error) {
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
		err    error
		rusage *syscall.Rusage
	}

	done := make(chan result, 1)
	go func() {
		runtime.LockOSThread()
		defer func() {
			if err := group.Destroy(); err != nil {
				fmt.Fprintln(os.Stderr, "goset: cgroup cleanup:", err)
			}
		}()
		defer runtime.UnlockOSThread()

		var prevCPUs cpu.CPUSet
		var gerr error
		if group == nil {
			prevCPUs, gerr = cpu.GetAffinity(0) // goset affinity
			if err := cpu.SetAffinity(0, cpus); err != nil {
				done <- result{err: fmt.Errorf("set thread affinity: %w", err)}
				return
			}
		}
		err := task.Run() // start and wait for the task to exit

		if group == nil && gerr == nil {
			_ = cpu.SetAffinity(0, prevCPUs)
		}
		var rusage *syscall.Rusage
		if task.ProcessState != nil {
			rusage, _ = task.ProcessState.SysUsage().(*syscall.Rusage)
		}
		done <- result{err, rusage}
	}()

	res := <-done
	return res.rusage, res.err
}
