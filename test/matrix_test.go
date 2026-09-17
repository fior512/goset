//go:build integration

// Root coverage of flag combinations. Each single flag already has its own
// dedicated test elsewhere (harness_test.go, selection_test.go, steer_test.go);
// this file exercises them pairwise/together so interactions between
// -include/-exclude/-steer/-numa-node/-cgroup(n>1)/-sampling-ms are not left
// untested.
package integration

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
		{"n1_exclude_steer", []string{"-n", "1", "-exclude", itoa(ids[len(ids)-1]), "-steer"}, 1},
		{"n2_numa_auto", []string{"-n", "2", "-numa-node", "-1"}, 2},
		{"n2_numa_off_steer", []string{"-n", "2", "-numa-node", "-2", "-steer"}, 2},
		{"n1_sampling0", []string{"-n", "1", "-sampling-ms", "0"}, 1},
		{"n2_sampling0_steer", []string{"-n", "2", "-sampling-ms", "0", "-steer"}, 2},
		{"n2_include_exclude_disjoint_steer", []string{
			"-n", "2",
			"-include", csv(ids[0], ids[1]),
			"-exclude", itoa(ids[len(ids)-1]),
			"-steer",
		}, 2},
		{"n1_cgroup_flag_noop", []string{"-n", "1", "-cgroup"}, 1}, // n=1: cgroup gated on n>1, flag itself is a no-op (see TestMultiThreadCgroupFlagIsDead)
		{"n2_cgroup_steer_numa", []string{
			"-n", "2", "-cgroup", "-steer", "-numa-node", "-1", "-sampling-ms", "10",
		}, 2},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Distinct binary name per case: InitCgroup derives the cgroup
			// directory from filepath.Base("goset-"+task path), which collapses
			// to the binary's own name regardless of its containing tempdir
			// (see cgroupSnapshot's doc comment in harness_test.go). Reusing one
			// shared probeBin across cases would make every subtest fight over
			// the same cgroup dir name and produce spurious leaked/missing diffs.
			probeBin := buildBin(t, root, "threadprobe-"+c.name, "./test/threadprobe")

			before := cgroupSnapshot(t)
			args := append([]string{}, c.args...)
			args = append(args, "--", probeBin, "-workers", itoa(max(c.wantCPU, 1)))
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
