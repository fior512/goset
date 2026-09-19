package integration

import (
	"goset/internal/generic"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func steerableIRQSnapshot(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(generic.ProcIRQ)
	if err != nil {
		t.Fatalf("read %s: %v", generic.ProcIRQ, err)
	}
	snap := map[string]string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := parseIntStrict(e.Name()); err != nil {
			continue
		}
		path := filepath.Join(generic.ProcIRQ, e.Name(), generic.SmpAffinityList)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		snap[e.Name()] = strings.TrimSpace(string(data))
	}
	return snap
}


func parseIntStrict(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, os.ErrInvalid
		}
		n = n*10 + int(r-'0')
	}
	if s == "" {
		return 0, os.ErrInvalid
	}
	return n, nil
}


func TestSteerRestoresIRQAffinity(t *testing.T) {
	preflight(t, 3) // n=1 + housekeeper + at least one CPU left to steer onto
	gosetBin, probeBin := setup(t)

	before := steerableIRQSnapshot(t)
	if len(before) == 0 {
		t.Skip("no steerable /proc/irq entries on this host")
	}

	res := runGoset(t, gosetBin, "-n", "1", "-steer", "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("goset -n 1 -steer exited %d, stderr: %s", res.exitCode, res.stderr)
	}

	after := steerableIRQSnapshot(t)
	for label, was := range before {
		now, ok := after[label]
		if !ok {
			continue
		}
		if now != was {
			t.Errorf("irq %s smp_affinity_list not restored: got %q, want %q", label, now, was)
		}
	}
}


func TestSteerCombinedWithCgroup(t *testing.T) {
	preflight(t, 4) // n=2 + housekeeper + at least one CPU left to steer onto
	gosetBin, probeBin := setup(t)

	before := cgroupSnapshot(t)
	res := runGoset(t, gosetBin, "-n", "2", "-steer", "--", probeBin, "-workers", "2")
	after := cgroupSnapshot(t)

	if res.exitCode != 0 {
		t.Fatalf("goset -n 2 -steer exited %d, stderr: %s", res.exitCode, res.stderr)
	}
	leaked, missing := diffSnapshots(before, after)
	if len(leaked) != 0 || len(missing) != 0 {
		t.Errorf("cgroup dir set not restored with -steer: leaked=%v missing=%v", leaked, missing)
	}

	rep := parseProbe(t, res.stdout)
	assertUniformAllowed(t, rep, 2)
}


func TestSteerNoRoomToSteer(t *testing.T) {
	preflight(t, 2)
	ids := onlineIDs(t)
	gosetBin, probeBin := setup(t)
	n := len(ids) - 1
	if n < 1 {
		t.Skip("need at least 2 online cpus")
	}

	res := runGoset(t, gosetBin, "-n", itoa(n), "-steer", "--", probeBin)
	if res.exitCode != 0 {
		t.Fatalf("goset -n %d -steer exited %d, stderr: %s", n, res.exitCode, res.stderr)
	}
}
