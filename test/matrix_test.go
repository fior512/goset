package integration

import "strconv"
import "testing"

func TestMatrixPermutations(t *testing.T) {
	ids := onlineIDs(t)
	preflight(t, 3)
	root := repoRoot(t)
	gosetBin := buildBin(t, root, "goset", "./cmd/goset")

	cases := []struct {
		name    string
		args    []string
		wantCPU int // 0: don't check exact count beyond >0
	}{
		{"n2_steer", []string{"-n", "2", "-steer"}, 2},
		{"n2_include2_steer", []string{"-n", "2", "-include", csv(ids[0], ids[1]), "-steer"}, 2},
		{"n1_exclude_steer", []string{"-n", "1", "-exclude", strconv.Itoa(ids[len(ids)-1]), "-steer"}, 1},
		{"n2_numa_auto", []string{"-n", "2", "-numa", "-1"}, 2},
		{"n2_numa_off_steer", []string{"-n", "2", "-numa", "-2", "-steer"}, 2},
		{"n1_sampling0", []string{"-n", "1", "-interval", "0"}, 1},
		{"n2_sampling0_steer", []string{"-n", "2", "-interval", "0", "-steer"}, 2},
		{"n2_include_exclude_disjoint_steer", []string{
			"-n", "2",
			"-include", csv(ids[0], ids[1]),
			"-exclude", strconv.Itoa(ids[len(ids)-1]),
			"-steer",
		}, 2},
		{"n1_cgroup_flag_noop", []string{"-n", "1", "-cgroup"}, 1}, // n=1: cgroup gated on n>1, flag itself is a no-op (see TestMultiThreadCgroupFlagIsDead)
		{"n2_cgroup_steer_numa", []string{
			"-n", "2", "-cgroup", "-steer", "-numa", "-1", "-interval", "10",
		}, 2},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			probeBin := buildBin(t, root, "threadprobe-"+c.name, "./test/threadprobe")

			before := cgroupSnapshot(t)
			args := append([]string{}, c.args...)
			args = append(args, "--", probeBin, "-workers", strconv.Itoa(max(c.wantCPU, 1)))
			res := runGoset(t, gosetBin, args...)
			after := cgroupSnapshot(t)

			if res.exitCode != 0 {
				t.Fatalf("args=%v exited %d, stderr: %s", c.args, res.exitCode, res.stderr)
			}
			leaked, missing := diffSnapshots(before, after)
			if len(leaked) != 0 || len(missing) != 0 {
				t.Errorf("args=%v: cgroup dir set not restored: leaked=%v missing=%v", c.args, leaked, missing)
			}
			rep := parseProbe(t, res.stdout)
			assertUniformAllowed(t, rep, c.wantCPU)
		})
	}
}
