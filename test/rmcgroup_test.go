package integration

import "testing"

func TestFlagRmCgroupNonexistent(t *testing.T) {
	gosetBin, _ := setup(t)
	res := runGoset(t, gosetBin, "-rm-cgroup", "goset-does-not-exist-12345")
	if res.exitCode == 0 {
		t.Fatal("expected error removing a nonexistent cgroup, got exit 0")
	}
}

func TestFlagRmCgroupTakesPathBase(t *testing.T) {
	gosetBin, _ := setup(t)
	res := runGoset(t, gosetBin, "-rm-cgroup", "/some/path/goset-does-not-exist-12345")
	if res.exitCode == 0 {
		t.Fatal("expected error removing a nonexistent cgroup, got exit 0")
	}
}
