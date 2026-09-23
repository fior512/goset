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

type TaskResult struct {
	Err    error
	Rusage *syscall.Rusage
	Wall   time.Duration
}

func ApplyPin(argv []string, cpus generic.CPUSet, group *Cgroup) *TaskResult {
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
	type launch struct {
		begin time.Time // t0 = the instant the task is exec'd
		err   error
	}
	started := make(chan launch, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if group != nil {
			err := task.Start()
			started <- launch{time.Now(), err}
			return
		}

		prevCPUs, gerr := cpu.GetAffinity(0) // goset affinity
		if err := cpu.SetAffinity(0, cpus); err != nil {
			started <- launch{err: fmt.Errorf("set thread affinity: %w", err)}
			return
		}
		err := task.Start()
		begin := time.Now() // after exec, before affinity restore
		if gerr == nil {
			_ = cpu.SetAffinity(0, prevCPUs)
		}
		started <- launch{begin, err}
	}()

	l := <-started
	if l.err != nil {
		return &TaskResult{Err: l.err}
	}

	err := task.Wait()
	res := &TaskResult{Err: err, Wall: time.Since(l.begin)}
	if task.ProcessState != nil {
		res.Rusage, _ = task.ProcessState.SysUsage().(*syscall.Rusage)
	}
	return res
}
