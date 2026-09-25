package integration

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"goset/internal/generic"
)

func reportWall(t *testing.T, stderr string) time.Duration {
	t.Helper()
	units := []struct {
		suffix string
		scale  time.Duration
	}{
		{"ns", time.Nanosecond},
		{"us", time.Microsecond},
		{"ms", time.Millisecond},
		{"s", time.Second},
	}
	inGlobal := false
	for _, line := range strings.Split(stderr, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			inGlobal = false
			continue
		}
		if trimmed == "Global" {
			inGlobal = true
			continue
		}
		if !inGlobal {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) != 2 || fields[0] != generic.GlobalWall {
			continue
		}
		for _, unit := range units {
			if !strings.HasSuffix(fields[1], unit.suffix) {
				continue
			}
			value, err := strconv.ParseFloat(strings.TrimSuffix(fields[1], unit.suffix), 64)
			if err != nil {
				t.Fatalf("parse %s value %q: %v", generic.GlobalWall, fields[1], err)
			}
			return time.Duration(value * float64(unit.scale))
		}
		t.Fatalf("%s value %q has no known unit", generic.GlobalWall, fields[1])
	}
	t.Fatalf("no %s row in the report:\n%s", generic.GlobalWall, stderr)
	return 0
}

func externalWall(t *testing.T, task []string) time.Duration {
	t.Helper()
	cmd := exec.Command(task[0], task[1:]...)
	start := time.Now()
	if err := cmd.Run(); err != nil {
		t.Fatalf("run %v: %v", task, err)
	}
	return time.Since(start)
}

func TestIsolatedWallMatchesExternalClock(t *testing.T) {
	const tolerance = 250 * time.Millisecond
	gosetBin := buildBin(t, repoRoot(t), "goset", "./cmd/goset")
	task := []string{"/bin/sh", "-c", "exec sleep 0.3"}

	external := externalWall(t, task)
	res := runGoset(t, gosetBin, append([]string{"-n", "1", "--"}, task...)...)
	if res.exitCode != 0 {
		t.Fatalf("goset -n 1 exited %d, stderr: %s", res.exitCode, res.stderr)
	}

	wall := reportWall(t, res.stderr)
	if wall < 300*time.Millisecond {
		t.Errorf("report %s = %v, want at least the 300ms task", generic.GlobalWall, wall)
	}
	if delta := wall - external; delta < -tolerance || delta > tolerance {
		t.Errorf("report %s = %v, external clock = %v, delta = %v, want within %v",
			generic.GlobalWall, wall, external, delta, tolerance)
	}
}
