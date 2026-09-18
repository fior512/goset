package integration

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"goset/internal/cpu"
	"goset/internal/isolation"
)

func cgroupSnapshot(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir("/sys/fs/cgroup")
	if err != nil {
		t.Fatalf("read /sys/fs/cgroup: %v", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}


func diffSnapshots(before, after []string) (leaked, missing []string) {
	seen := map[string]bool{}
	for _, n := range after {
		seen[n] = true
	}
	for _, n := range before {
		delete(seen, n)
	}
	for n := range seen {
		leaked = append(leaked, n)
	}
	afterSet := map[string]bool{}
	for _, n := range after {
		afterSet[n] = true
	}
	for _, n := range before {
		if !afterSet[n] {
			missing = append(missing, n)
		}
	}
	return leaked, missing
}


func assertUniformAllowed(t *testing.T, rep probeReport, wantCPUs int) {
	t.Helper()
	if len(rep.Threads) == 0 {
		t.Fatal("threadprobe reported zero threads")
	}
	first := rep.Threads[0].Allowed
	for _, th := range rep.Threads {
		if th.Err != "" {
			t.Errorf("thread %d probe error: %s", th.TID, th.Err)
			continue
		}
		if th.Allowed != first {
			t.Errorf("thread %d allowed=%q, want %q (all threads must share the pinned set)", th.TID, th.Allowed, first)
		}
		t.Logf("thread %d: allowed=%s observed=cpu%d", th.TID, th.Allowed, th.Observed)
	}
	got := cpuListCount(first)
	if got != wantCPUs {
		t.Errorf("allowed set %q has %d cpu(s), want %d", first, got, wantCPUs)
	}
}


func TestSingleThreadNoCgroup(t *testing.T) {
	preflight(t, 2) // n=1 + housekeeper
	root := repoRoot(t)
	gosetBin := buildBin(t, root, "goset", "./cmd/goset")
	probeBin := buildBin(t, root, "threadprobe", "./test/threadprobe")

	before := cgroupSnapshot(t)
	res := runGoset(t, gosetBin, "-n", "1", "--", probeBin, "-workers", "1")
	after := cgroupSnapshot(t)

	if res.exitCode != 0 {
		t.Fatalf("goset -n 1 exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	leaked, missing := diffSnapshots(before, after)
	if len(leaked) != 0 || len(missing) != 0 {
		t.Errorf("cgroup set changed for -n 1 (no cgroup expected): leaked=%v missing=%v", leaked, missing)
	}

	rep := parseProbe(t, res.stdout)
	assertUniformAllowed(t, rep, 1)
}


func TestMultiThreadCgroupDestroyed(t *testing.T) {
	preflight(t, 3) // n=2 + housekeeper
	root := repoRoot(t)
	gosetBin := buildBin(t, root, "goset", "./cmd/goset")
	probeBin := buildBin(t, root, "threadprobe", "./test/threadprobe")

	before := cgroupSnapshot(t)
	res := runGoset(t, gosetBin, "-n", "2", "--", probeBin, "-workers", "4")
	after := cgroupSnapshot(t)

	if res.exitCode != 0 {
		t.Fatalf("goset -n 2 exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	leaked, missing := diffSnapshots(before, after)
	if len(leaked) != 0 || len(missing) != 0 {
		t.Errorf("cgroup dir set not restored after run: leaked=%v missing=%v (see InitCgroup/Destroy in internal/isolation/cgroup.go)", leaked, missing)
	}

	rep := parseProbe(t, res.stdout)
	assertUniformAllowed(t, rep, 2)
	if rep.Cgroup == "" || strings.Contains(rep.Cgroup, "0::/\n") || strings.HasSuffix(rep.Cgroup, "0::/") {
		t.Errorf("expected task to run inside a non-root cgroup, got %q", rep.Cgroup)
	} else {
		t.Logf("task ran in cgroup: %s", rep.Cgroup)
	}
}


func TestMultiThreadCgroupFlagIsDead(t *testing.T) {
	preflight(t, 3)
	root := repoRoot(t)
	gosetBin := buildBin(t, root, "goset", "./cmd/goset")
	probeBin := buildBin(t, root, "threadprobe", "./test/threadprobe")

	before := cgroupSnapshot(t)
	res := runGoset(t, gosetBin, "-n", "2", "-cgroup", "--", probeBin, "-workers", "1")
	after := cgroupSnapshot(t)

	if res.exitCode != 0 {
		t.Fatalf("goset -n 2 -cgroup exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	leaked, missing := diffSnapshots(before, after)
	if len(leaked) != 0 || len(missing) != 0 {
		t.Errorf("cgroup dir set not restored: leaked=%v missing=%v", leaked, missing)
	}
	rep := parseProbe(t, res.stdout)
	assertUniformAllowed(t, rep, 2)
}


func TestErrorMissingTaskBinary(t *testing.T) {
	preflight(t, 2)
	root := repoRoot(t)
	gosetBin := buildBin(t, root, "goset", "./cmd/goset")

	before := cgroupSnapshot(t)
	res := runGoset(t, gosetBin, "-n", "1", "--", "/nonexistent/path/to/nothing")
	after := cgroupSnapshot(t)

	if res.exitCode == 0 {
		t.Fatalf("expected non-zero exit for missing task binary, got 0 (stdout: %s)", res.stdout)
	}
	if !strings.Contains(res.stderr, "goset:") {
		t.Errorf("stderr missing user-facing prefix %q: %s", "goset:", res.stderr)
	}
	leaked, missing := diffSnapshots(before, after)
	if len(leaked) != 0 || len(missing) != 0 {
		t.Errorf("cgroup dir set changed on a failed run: leaked=%v missing=%v", leaked, missing)
	}
}


func TestErrorNRequestTooLarge(t *testing.T) {
	preflight(t, 1)
	root := repoRoot(t)
	gosetBin := buildBin(t, root, "goset", "./cmd/goset")
	probeBin := buildBin(t, root, "threadprobe", "./test/threadprobe")

	res := runGoset(t, gosetBin, "-n", "100000", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatalf("expected non-zero exit for oversized -n, got 0")
	}
	if !strings.Contains(res.stderr, "cgroup") {
		t.Errorf("stderr should mention the missing -cgroup requirement first, got: %s", res.stderr)
	}

	res = runGoset(t, gosetBin, "-n", "100000", "-cgroup", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatalf("expected non-zero exit for oversized -n even with -cgroup, got 0")
	}
	if !strings.Contains(res.stderr, "housekeeper") {
		t.Errorf("stderr should explain the housekeeper+n requirement once -cgroup is set, got: %s", res.stderr)
	}
}


func TestRmCgroupRemovesLeaked(t *testing.T) {
	preflight(t, 2)
	root := repoRoot(t)
	gosetBin := buildBin(t, root, "goset", "./cmd/goset")

	name := "goset-rmcgroup-test"
	var cpus cpu.CPUSet
	cpus.SetBit(0)
	group, err := isolation.InitCgroup(name, cpus, -1)
	if err != nil {
		t.Fatalf("InitCgroup setup: %v", err)
	}
	group.File.Close() // avoid leaking the fd if the test fails before removal

	res := runGoset(t, gosetBin, "-rm-cgroup", name)
	if res.exitCode != 0 {
		t.Fatalf("-rm-cgroup %s exited %d, stderr: %s", name, res.exitCode, res.stderr)
	}
	if _, err := os.Stat(filepath.Join("/sys/fs/cgroup", name)); !os.IsNotExist(err) {
		t.Errorf("cgroup dir still present after -rm-cgroup: %v", err)
	}
}


func TestErrorIncludeExcludeOverlap(t *testing.T) {
	preflight(t, 1)
	root := repoRoot(t)
	gosetBin := buildBin(t, root, "goset", "./cmd/goset")
	probeBin := buildBin(t, root, "threadprobe", "./test/threadprobe")

	res := runGoset(t, gosetBin, "-include", "0", "-exclude", "0", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatalf("expected non-zero exit for overlapping -include/-exclude, got 0")
	}
	if !strings.Contains(res.stderr, "overlap") {
		t.Errorf("stderr should mention the include/exclude overlap, got: %s", res.stderr)
	}
}
