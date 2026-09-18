// Unprivileged coverage of -interval values that pass Config.Validate
// (>= -1). Uses -n 1 so no cgroup and no root are needed; only checks the
// run still succeeds and produces a probe report, since the sampler window
// itself is not observable from threadprobe's output.
package integration

import "testing"

func TestFlagIntervalValues(t *testing.T) {
	gosetBin, probeBin := setup(t)
	for _, v := range []string{"-1", "0", "1", "50"} {
		res := runGoset(t, gosetBin, "-n", "1", "-interval", v, "--", probeBin)
		if res.exitCode != 0 {
			t.Errorf("-interval %s: exited %d, stderr: %s", v, res.exitCode, res.stderr)
			continue
		}
		parseProbe(t, res.stdout)
	}
}
