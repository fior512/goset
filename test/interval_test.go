package integration

import "testing"

func TestFlagIntervalValues(t *testing.T) {
	gosetBin, probeBin := setup(t)
	for _, v := range []string{"0", "1", "50"} {
		res := runGoset(t, gosetBin, "-n", "1", "-interval", v, "--", probeBin)
		if res.exitCode != 0 {
			t.Errorf("-interval %s: exited %d, stderr: %s", v, res.exitCode, res.stderr)
			continue
		}
		parseProbe(t, res.stdout)
	}
}
