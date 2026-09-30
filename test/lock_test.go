package integration

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"goset/internal/generic"
	"goset/internal/isolation"
)

func TestLockRejectsSecondHolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goset-test.lock")

	lock, err := isolation.AcquireLock(path)
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}

	_, err = isolation.AcquireLock(path)
	if err == nil {
		lock.Release()
		t.Fatal("second AcquireLock succeeded while held")
	}
	if want := "pid " + strconv.Itoa(os.Getpid()); !strings.Contains(err.Error(), want) {
		t.Errorf("second AcquireLock error = %q, want it to contain %q", err, want)
	}

	lock.Release()
	again, err := isolation.AcquireLock(path)
	if err != nil {
		t.Fatalf("AcquireLock after Release: %v", err)
	}
	again.Release()
}

func TestLockReleaseNilSafe(t *testing.T) {
	var lock *isolation.Lock
	lock.Release()
	lock.Release()
}

func TestConcurrentCgroupRunRejected(t *testing.T) {
	preflight(t, 3)
	gosetBin, probeBin := setup(t)

	lock, err := isolation.AcquireLock(filepath.Join(generic.RunLockDir, "goset-threadprobe.lock"))
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer lock.Release()

	res := runGoset(t, gosetBin, "-n", "2", "-cgroup", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatalf("goset -cgroup succeeded while the cgroup lock was held, stderr: %s", res.stderr)
	}
	if !strings.Contains(res.stderr, "lock") {
		t.Errorf("stderr = %q, want it to mention the lock", res.stderr)
	}
}

func TestConcurrentSteerRunRejected(t *testing.T) {
	// -n 1 plus the housekeeper plus the steered cpu, or selection refuses
	// before the lock is reached
	preflight(t, 3)
	gosetBin, probeBin := setup(t)

	lock, err := isolation.AcquireLock(filepath.Join(generic.RunLockDir, generic.SteerLockName))
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer lock.Release()

	res := runGoset(t, gosetBin, "-n", "1", "-steer", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatalf("goset -steer succeeded while the steer lock was held, stderr: %s", res.stderr)
	}
	if !strings.Contains(res.stderr, "lock") {
		t.Errorf("stderr = %q, want it to mention the lock", res.stderr)
	}
}
