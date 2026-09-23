package isolation

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"goset/internal/cpu"
	"goset/internal/generic"
)

func ApplyPin(argv []string, cpus generic.CPUSet, group *Cgroup) (*syscall.Rusage, time.Duration, error) {
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
	var wall time.Duration
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		var prevCPUs generic.CPUSet
		var gerr error
		if group == nil {
			prevCPUs, gerr = cpu.GetAffinity(0) // goset affinity
			if err := cpu.SetAffinity(0, cpus); err != nil {
				done <- fmt.Errorf("set thread affinity: %w", err)
				return
			}
		}

		if err := task.Start(); err != nil {
			done <- err
			return
		}
		t0 := time.Now() // the instant the task is exec'd
		if group == nil && gerr == nil {
			_ = cpu.SetAffinity(0, prevCPUs)
		}

		err := task.Wait()
		wall = time.Since(t0) // Wait() return = process reaped
		done <- err
	}()

	err := <-done
	var rusage *syscall.Rusage
	if task.ProcessState != nil {
		rusage, _ = task.ProcessState.SysUsage().(*syscall.Rusage)
	}
	return rusage, wall, err
}
