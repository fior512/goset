package isolation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"goset/internal/cpu"
	"goset/internal/generic"
)

const taskKillDelay = 3 * time.Second

func ApplyPin(ctx context.Context, argv []string, cpus generic.CPUSet, group *Cgroup) (*syscall.Rusage, time.Duration, error) {
	// Task In/Out
	task := exec.CommandContext(ctx, argv[0], argv[1:]...)
	task.Stdin = os.Stdin
	task.Stdout = os.Stdout
	task.Stderr = os.Stderr
	task.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	task.Cancel = func() error {
		err := syscall.Kill(-task.Process.Pid, syscall.SIGINT) // group
		if err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		return nil
	}
	task.WaitDelay = taskKillDelay

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

		t0 := time.Now() // launch instant, wall covers fork and exec
		if err := task.Start(); err != nil {
			done <- err
			return
		}
		if group == nil && gerr == nil {
			_ = cpu.SetAffinity(0, prevCPUs)
		}
		if group != nil {
			if err := pinTask(task.Process.Pid, cpus); err != nil {
				_ = task.Process.Kill()
				_ = task.Wait()
				done <- fmt.Errorf("pin task inside cgroup: %w", err)
				return
			}
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

func pinTask(pid int, cpus generic.CPUSet) error {
	/* list */
	entries, err := os.ReadDir(filepath.Join(generic.ProcRoot, strconv.Itoa(pid), generic.ProcTaskDir))
	if err != nil {
		return err
	}

	/* pin */
	for _, entry := range entries {
		tid, err := strconv.Atoi(entry.Name())
		if err != nil {
			return fmt.Errorf("tid %q: %w", entry.Name(), err)
		}
		// ESRCH: the thread exited after the listing
		if err := cpu.SetAffinity(tid, cpus); err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("tid %d: %w", tid, err)
		}
	}
	return nil
}
