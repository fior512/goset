package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goset/internal/generic"
)

func TestSubtreeControlRestored(t *testing.T) {
	preflight(t, 3)
	gosetBin, probeBin := setup(t)

	subtree := filepath.Join(generic.SysCgroup, generic.CgroupSubtreeControl)
	before, err := os.ReadFile(subtree)
	if err != nil {
		t.Fatalf("read %s: %v", subtree, err)
	}

	res := runGoset(t, gosetBin, "-n", "2", "-cgroup", "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("goset -n 2 -cgroup exited %d, stderr: %s", res.exitCode, res.stderr)
	}

	after, err := os.ReadFile(subtree)
	if err != nil {
		t.Fatalf("read %s: %v", subtree, err)
	}
	if string(after) != string(before) {
		t.Errorf("root subtree_control = %q, want the pre run value %q",
			strings.TrimSpace(string(after)), strings.TrimSpace(string(before)))
	}
}
