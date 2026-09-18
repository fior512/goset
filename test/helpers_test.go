package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func isSudo(t *testing.T) bool {
	t.Helper()
	return os.Geteuid() == 0
}


type threadInfo struct {
	TID      int    `json:"tid"`
	Allowed  string `json:"allowed"`
	Observed int    `json:"observed"`
	Err      string `json:"err,omitempty"`
}


type probeReport struct {
	PID     int          `json:"pid"`
	Workers int          `json:"workers"`
	Cgroup  string       `json:"cgroup"`
	Threads []threadInfo `json:"threads"`
}


func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filepath.Dir(file))
}


func buildBin(t *testing.T, root, outName, pkg string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), outName)
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build %s: %v\n%s", pkg, err, stderr.String())
	}
	return out
}


type runResult struct {
	stdout   string
	stderr   string
	exitCode int
}


func runGoset(t *testing.T, gosetBin string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(gosetBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run goset: %v (stderr: %s)", err, stderr.String())
		}
	}
	return runResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: code}
}


func parseProbe(t *testing.T, stdout string) probeReport {
	t.Helper()
	var rep probeReport
	dec := json.NewDecoder(strings.NewReader(stdout))
	if err := dec.Decode(&rep); err != nil {
		t.Fatalf("parse threadprobe json: %v\nraw stdout: %s", err, stdout)
	}
	return rep
}


func cpuListCount(list string) int {
	if list == "" {
		return 0
	}
	n := 0
	for _, part := range strings.Split(list, ",") {
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			var a, b int
			fmt.Sscanf(lo, "%d", &a)
			fmt.Sscanf(hi, "%d", &b)
			n += b - a + 1
		} else {
			n++
		}
	}
	return n
}


// parseCPUList turns "0,2-4" into the sorted ids [0 2 3 4] for test
// assertions that need to inspect individual members.
func parseCPUList(t *testing.T, list string) []int {
	t.Helper()
	var out []int
	if list == "" {
		return out
	}
	for _, part := range strings.Split(list, ",") {
		if lo, hi, ok := strings.Cut(part, "-"); ok {
			var a, b int
			fmt.Sscanf(lo, "%d", &a)
			fmt.Sscanf(hi, "%d", &b)
			for id := a; id <= b; id++ {
				out = append(out, id)
			}
		} else {
			var id int
			fmt.Sscanf(part, "%d", &id)
			out = append(out, id)
		}
	}
	return out
}
