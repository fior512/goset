package integration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"goset/internal/generic"
)

type signaledRun struct {
	cmd    *exec.Cmd
	stderr bytes.Buffer
}

func startGoset(t *testing.T, gosetBin string, args ...string) *signaledRun {
	t.Helper()
	run := &signaledRun{cmd: exec.Command(gosetBin, args...)}
	run.cmd.Stderr = &run.stderr
	if err := run.cmd.Start(); err != nil {
		t.Fatalf("start goset: %v", err)
	}
	return run
}

func (run *signaledRun) signal(t *testing.T, sig os.Signal) {
	t.Helper()
	if err := run.cmd.Process.Signal(sig); err != nil {
		t.Fatalf("signal goset: %v", err)
	}
}

func (run *signaledRun) wait(t *testing.T) int {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- run.cmd.Wait() }()
	var err error
	select {
	case err = <-done:
	case <-time.After(30 * time.Second):
		_ = run.cmd.Process.Kill()
		t.Fatalf("goset did not exit within 30s, stderr: %s", run.stderr.String())
	}
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("wait goset: %v (stderr: %s)", err, run.stderr.String())
		}
		return exitErr.ExitCode()
	}
	return 0
}

func sleeperTask(ready string) []string {
	return []string{"/bin/sh", "-c", fmt.Sprintf("touch %s; exec sleep 30", ready)}
}

func waitTaskReady(t *testing.T, ready string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("task did not start: %s missing", ready)
}

func gosetBinary(t *testing.T) string {
	t.Helper()
	return buildBin(t, repoRoot(t), "goset", "./cmd/goset")
}

func TestSigtermRunsTeardown(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	run := startGoset(t, gosetBinary(t), append([]string{"-n", "1", "--"}, sleeperTask(ready)...)...)

	waitTaskReady(t, ready)
	run.signal(t, syscall.SIGTERM)

	if code := run.wait(t); code != 130 {
		t.Fatalf("exit code = %d, want 130 from the forwarded SIGINT, stderr: %s", code, run.stderr.String())
	}
	if !strings.Contains(run.stderr.String(), generic.RunExit) {
		t.Errorf("stderr = %q, want the goset report with %q", run.stderr.String(), generic.RunExit)
	}
}

func TestSigintRunsTeardown(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	run := startGoset(t, gosetBinary(t), append([]string{"-n", "1", "--"}, sleeperTask(ready)...)...)

	waitTaskReady(t, ready)
	run.signal(t, syscall.SIGINT)

	if code := run.wait(t); code != 130 {
		t.Fatalf("exit code = %d, want 130 from the forwarded SIGINT, stderr: %s", code, run.stderr.String())
	}
	if !strings.Contains(run.stderr.String(), generic.RunExit) {
		t.Errorf("stderr = %q, want the goset report with %q", run.stderr.String(), generic.RunExit)
	}
}

func TestSigintRestoresCgroupAndIRQs(t *testing.T) {
	preflight(t, 3)
	beforeGroups := cgroupSnapshot(t)
	beforeIRQs := steerableIRQSnapshot(t)

	ready := filepath.Join(t.TempDir(), "ready")
	run := startGoset(t, gosetBinary(t), append([]string{"-n", "2", "-cgroup", "-steer", "--"}, sleeperTask(ready)...)...)

	waitTaskReady(t, ready)
	run.signal(t, syscall.SIGINT)

	if code := run.wait(t); code != 130 {
		t.Fatalf("exit code = %d, want 130 from the forwarded SIGINT, stderr: %s", code, run.stderr.String())
	}
	leaked, missing := diffSnapshots(beforeGroups, cgroupSnapshot(t))
	if len(leaked) > 0 {
		t.Errorf("leaked cgroups: %v", leaked)
	}
	if len(missing) > 0 {
		t.Errorf("missing cgroups: %v", missing)
	}
	afterIRQs := steerableIRQSnapshot(t)
	for label, want := range beforeIRQs {
		if got, ok := afterIRQs[label]; ok && got != want {
			t.Errorf("irq %s affinity = %q, want %q", label, got, want)
		}
	}
}
