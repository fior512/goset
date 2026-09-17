// Unprivileged coverage of -rm-cgroup against a name that was never
// created: RemoveCgroup (internal/isolation/cgroup.go) must fail cleanly
// rather than panic or exit 0. The success path (removing a cgroup that
// really exists) needs root to create one first; see the integration tier.
package integration

import "testing"

func TestFlagRmCgroupNonexistent(t *testing.T) {
	gosetBin, _ := setup(t)
	res := runGoset(t, gosetBin, "-rm-cgroup", "goset-does-not-exist-12345")
	if res.exitCode == 0 {
		t.Fatal("expected error removing a nonexistent cgroup, got exit 0")
	}
}

// TestFlagRmCgroupTakesPathBase pins that -rm-cgroup derives the target
// directory via filepath.Base, same as InitCgroup, so a path-shaped name
// still resolves under /sys/fs/cgroup rather than being rejected outright.
func TestFlagRmCgroupTakesPathBase(t *testing.T) {
	gosetBin, _ := setup(t)
	res := runGoset(t, gosetBin, "-rm-cgroup", "/some/path/goset-does-not-exist-12345")
	if res.exitCode == 0 {
		t.Fatal("expected error removing a nonexistent cgroup, got exit 0")
	}
}
