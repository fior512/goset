package integration

import (
	"strings"
	"testing"
)

func TestFlagNZeroAccepted(t *testing.T) {
	gosetBin, probeBin := setup(t)
	res := runGoset(t, gosetBin, "-n", "0", "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("expected -n 0 to succeed, exited %d, stderr: %s", res.exitCode, res.stderr)
	}
}

func TestFlagNZeroWithCgroupRejected(t *testing.T) {
	gosetBin, probeBin := setup(t)
	res := runGoset(t, gosetBin, "-n", "0", "-cgroup", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatal("expected error for -n 0 -cgroup: no cpus to put in the cpuset")
	}
	if !strings.Contains(res.stderr, "cgroup") {
		t.Errorf("stderr should mention cgroup, got: %s", res.stderr)
	}
}

func TestFlagNNegative(t *testing.T) {
	gosetBin, probeBin := setup(t)
	res := runGoset(t, gosetBin, "-n", "-1", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatal("expected error for -n -1")
	}
}

func TestFlagSteerWithoutRoot(t *testing.T) {
	if isSudo(t) {
		t.Skip("running as root: -steer root check not exercised")
	}
	gosetBin, probeBin := setup(t)
	res := runGoset(t, gosetBin, "-n", "1", "-steer", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatal("expected error for -steer without root")
	}
	if !strings.Contains(res.stderr, "root") {
		t.Errorf("stderr should mention root, got: %s", res.stderr)
	}
}

func TestFlagCgroupWithoutRoot(t *testing.T) {
	if isSudo(t) {
		t.Skip("running as root: -cgroup root check not exercised")
	}
	gosetBin, probeBin := setup(t)
	res := runGoset(t, gosetBin, "-n", "1", "-cgroup", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatal("expected error for -cgroup without root")
	}
	if !strings.Contains(res.stderr, "root") {
		t.Errorf("stderr should mention root, got: %s", res.stderr)
	}
}

func TestFlagNodeTooLow(t *testing.T) {
	gosetBin, probeBin := setup(t)
	res := runGoset(t, gosetBin, "-n", "1", "-node", "-3", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatal("expected error for -node -3")
	}
	if !strings.Contains(res.stderr, "node") {
		t.Errorf("stderr should mention node, got: %s", res.stderr)
	}
}

func TestFlagIntervalTooLow(t *testing.T) {
	gosetBin, probeBin := setup(t)
	res := runGoset(t, gosetBin, "-n", "1", "-interval", "-2", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatal("expected error for -interval -2")
	}
	if !strings.Contains(res.stderr, "interval") {
		t.Errorf("stderr should mention interval, got: %s", res.stderr)
	}
}

func TestFlagIncludeExcludeOverlapNoRoot(t *testing.T) {
	gosetBin, probeBin := setup(t)
	res := runGoset(t, gosetBin, "-n", "1", "-include", "0", "-exclude", "0", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatal("expected error for overlapping -include/-exclude")
	}
	if !strings.Contains(res.stderr, "overlap") {
		t.Errorf("stderr should mention overlap, got: %s", res.stderr)
	}
}

func TestFlagIncludeMalformed(t *testing.T) {
	gosetBin, probeBin := setup(t)
	for _, bad := range []string{"x", "1-", "-a"} {
		res := runGoset(t, gosetBin, "-n", "1", "-include", bad, "--", probeBin)
		if res.exitCode == 0 {
			t.Errorf("-include %q: expected error, got exit 0", bad)
		}
	}
}

func TestFlagExcludeMalformed(t *testing.T) {
	gosetBin, probeBin := setup(t)
	for _, bad := range []string{"x", "1-", "-a"} {
		res := runGoset(t, gosetBin, "-n", "1", "-exclude", bad, "--", probeBin)
		if res.exitCode == 0 {
			t.Errorf("-exclude %q: expected error, got exit 0", bad)
		}
	}
}

func TestFlagUnknown(t *testing.T) {
	gosetBin, probeBin := setup(t)
	res := runGoset(t, gosetBin, "-n", "1", "-does-not-exist", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatal("expected error for unknown flag")
	}
}

func TestFlagNodeValidValues(t *testing.T) {
	// -2 (off, default), -1 (auto), 0 (node idx) must all pass Validate.
	// NumaNode is not yet enforced by SelectCPUs (see internal/cpu/selection.go
	// TODO), so this only pins that the flag is accepted, not that it filters.
	gosetBin, probeBin := setup(t)
	for _, v := range []string{"-2", "-1", "0"} {
		res := runGoset(t, gosetBin, "-n", "1", "-node", v, "--", probeBin)
		if res.exitCode != 0 {
			t.Errorf("-node %s: exited %d, stderr: %s", v, res.exitCode, res.stderr)
		}
	}
}
