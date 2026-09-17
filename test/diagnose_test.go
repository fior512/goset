// Unprivileged coverage of the bare-invocation path (no task after "--"),
// which dispatches to runner.Diagnose (internal/runner/diagnose.go). Does
// not need root: GetTopology/GetEnvironment/ListCgroups all read files that
// are world-readable, and an empty /sys/fs/cgroup goset- listing is valid.
package integration

import (
	"strings"
	"testing"
)

func TestDiagnoseBareInvocation(t *testing.T) {
	gosetBin, _ := setup(t)
	res := runGoset(t, gosetBin)
	if res.exitCode != 0 {
		t.Fatalf("bare goset exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	for _, want := range []string{"Topology", "Environment", "Cgroups"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("diagnose output missing %q table, got: %s", want, res.stdout)
		}
	}
}

// TestDiagnoseRejectsInvalidSelectionFlags: with no task, goset still runs
// flags through Validate before dispatching to Diagnose, so an -n>1 without
// -cgroup errors out exactly as it would with a task.
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

// TestDiagnoseInvalidFlagsStillValidated: an invalid flag combination must
// still error before Diagnose runs, even with no task given.
func TestDiagnoseInvalidFlagsStillValidated(t *testing.T) {
	gosetBin, _ := setup(t)
	res := runGoset(t, gosetBin, "-n", "0")
	if res.exitCode == 0 {
		t.Fatal("expected error for -n 0 even with no task (diagnose path)")
	}
}
