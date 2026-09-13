package isolation

import (
	"fmt"
	"goset/internal/cpu"
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

func ApplyPin(argv []string, cpus cpu.CPUSet) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{}

	type result struct {
		err error
	}
	done := make(chan result, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		prevCpus, gerr := cpu.GetAffinity(0) // affinity caller thread
		if err := cpu.SetAffinity(0, cpus); err != nil {
			done <- result{fmt.Errorf("set thread affinity: %w", err)}
			return
		}

		err := cmd.Start() // run bash

		if gerr == nil {
			_ = cpu.SetAffinity(0, prevCpus)
		}
		done <- result{err}
	}()

	if r := <-done; r.err != nil {
		return r.err
	}

	return nil
}

