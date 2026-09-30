package integration

import (
	"strings"
	"testing"

	"goset/internal/generic"
)

func TestFlagNZeroRejected(t *testing.T) {
	gosetBin, probeBin := setup(t)
	res := runGoset(t, gosetBin, "-n", "0", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatal("expected error for -n 0")
	}
}


func TestFlagNZeroWithCgroupRejected(t *testing.T) {
	gosetBin, probeBin := setup(t)
	res := runGoset(t, gosetBin, "-n", "0", "-cgroup", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatal("expected error for -n 0 -cgroup")
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
	res := runGoset(t, gosetBin, "-n", "1", "-numa", "-3", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatal("expected error for -numa -3")
	}
	if !strings.Contains(res.stderr, "numa") {
		t.Errorf("stderr should mention numa, got: %s", res.stderr)
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


func TestFlagHelp(t *testing.T) {
	gosetBin, _ := setup(t)
	res := runGoset(t, gosetBin, "-h")
	if res.exitCode != 0 {
		t.Fatalf("-h: got exit %d, want 0", res.exitCode)
	}
	if !strings.Contains(res.stderr, "-interval") {
		t.Errorf("-h: no usage on stderr: %s", res.stderr)
	}
	if strings.Contains(res.stderr, generic.LogPrefix) {
		t.Errorf("-h: usage re-emitted as an error: %s", res.stderr)
	}
}


func TestFlagFenceWithoutCgroupRejected(t *testing.T) {
	gosetBin, probeBin := setup(t)
	res := runGoset(t, gosetBin, "-n", "1", "-fence", "--", probeBin)
	if res.exitCode == 0 {
		t.Fatal("expected error for -fence without -cgroup")
	}
	if !strings.Contains(res.stderr, "-cgroup") {
		t.Errorf("stderr should mention -cgroup, got: %s", res.stderr)
	}
}
