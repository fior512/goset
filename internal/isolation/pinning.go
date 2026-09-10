package isolation

import (
	"os"
	"os/exec"
	"syscall"
)

func ApplyPin(argv []string) error {
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
		err := cmd.Start()
		done <- result{err}
	}()

	if r := <-done; r.err != nil {
		return r.err
	}

	return nil
}
