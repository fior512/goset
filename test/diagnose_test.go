package integration

import (
	"strings"
	"testing"
)

func TestDiagnoseRejectsInvalidSelectionFlags(t *testing.T) {
	gosetBin, _ := setup(t)
	res := runGoset(t, gosetBin, "-include", "0", "-exclude", "0")
	if res.exitCode == 0 {
		t.Fatalf("expected error for bare goset with cpu 0 in both -include and -exclude, got exit 0 (stdout: %s)", res.stdout)
	}
	if !strings.Contains(res.stderr, "appear in both") {
		t.Errorf("stderr should report the overlap, got: %s", res.stderr)
	}
}

func TestDiagnoseInvalidFlagsStillValidated(t *testing.T) {
	gosetBin, _ := setup(t)
	res := runGoset(t, gosetBin, "-n", "-1")
	if res.exitCode == 0 {
		t.Fatal("expected error for -n -1 even with no task (diagnose path)")
	}
}
