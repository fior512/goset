package integration

import (
	"testing"

	"goset/internal/cli"
)

func TestParseMultiThreadWithoutCgroup(t *testing.T) {
	cfg, err := cli.Parse([]string{"-n", "5", "--", "./task"})
	if err != nil {
		t.Fatalf("-n 5 without -cgroup: %v", err)
	}
	if cfg.NThreads != 5 {
		t.Errorf("NThreads = %d, want 5", cfg.NThreads)
	}
}
