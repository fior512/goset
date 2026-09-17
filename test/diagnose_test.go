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

// TestDiagnoseIgnoresTaskSelectionFlags: with no task, goset dispatches to
// Diagnose regardless of what selection flags were also passed, since main.go
// only checks len(cfg.Task) == 0. Flags still go through Validate first.
func TestDiagnoseIgnoresTaskSelectionFlags(t *testing.T) {
	gosetBin, _ := setup(t)
	res := runGoset(t, gosetBin, "-n", "2", "-include", "0")
	if res.exitCode != 0 {
		t.Fatalf("bare goset with selection flags exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	if !strings.Contains(res.stdout, "Topology") {
		t.Errorf("diagnose output missing Topology table, got: %s", res.stdout)
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
