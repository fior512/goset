package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"goset/internal/generic"
	"goset/internal/isolation"
)

func foreignCgroup(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(generic.SysCgroup, name)
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Skipf("cannot create %s: %v", path, err)
	}
	t.Cleanup(func() { os.Remove(path) })
	return path
}

func oneCPU(t *testing.T) generic.CPUSet {
	t.Helper()
	var cpus generic.CPUSet
	cpus.SetBit(0)
	return cpus
}

func TestInitCgroupRefusesForeignExisting(t *testing.T) {
	preflight(t, 2)
	path := foreignCgroup(t, "adopt-foreign-test")

	if _, err := isolation.InitCgroup("adopt-foreign-test", oneCPU(t), -1); err == nil {
		t.Fatalf("InitCgroup adopted the foreign %s", path)
	} else if !strings.Contains(err.Error(), generic.CgroupIdentifier) {
		t.Errorf("InitCgroup error = %q, want it to name the %s prefix", err, generic.CgroupIdentifier)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("foreign cgroup %s gone: %v", path, err)
	}
}

func TestInitCgroupRefusesCgroupWithTasks(t *testing.T) {
	preflight(t, 2)
	name := generic.CgroupIdentifier + "adopt-busy-test"
	path := foreignCgroup(t, name)

	child := exec.Command("/bin/sh", "-c", "exec sleep 30")
	if err := child.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	defer func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	}()
	procs := filepath.Join(path, generic.CgroupProcs)
	if err := os.WriteFile(procs, []byte(strconv.Itoa(child.Process.Pid)), 0o644); err != nil {
		t.Fatalf("write %s: %v", procs, err)
	}

	if _, err := isolation.InitCgroup(name, oneCPU(t), -1); err == nil {
		t.Fatalf("InitCgroup adopted %s while it held a task", path)
	} else if !strings.Contains(err.Error(), "leftover") {
		t.Errorf("InitCgroup error = %q, want it to say the cgroup is not a leftover", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("busy cgroup %s gone: %v", path, err)
	}
}
