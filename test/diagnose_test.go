package integration

import (
	"strings"
	"testing"
)

func TestDiagnoseRejectsInvalidSelectionFlags(t *testing.T) {
	gosetBin, _ := setup(t)
	res := runGoset(t, gosetBin, "-n", "2", "-include", "0")
	if res.exitCode == 0 {
		t.Fatalf("expected error for bare goset with -n 2 and no -cgroup, got exit 0 (stdout: %s)", res.stdout)
	}
	if !strings.Contains(res.stderr, "cgroup") {
		t.Errorf("stderr should mention cgroup, got: %s", res.stderr)
	}
}

func TestDiagnoseInvalidFlagsStillValidated(t *testing.T) {
	gosetBin, _ := setup(t)
	res := runGoset(t, gosetBin, "-n", "-1")
	if res.exitCode == 0 {
		t.Fatal("expected error for -n -1 even with no task (diagnose path)")
	}
}
