package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goset/internal/cpu"
)

func preflight(t *testing.T, minCPUs int) {
	t.Helper()

	check := func(name string, ok bool, remedy string) {
		if ok {
			fmt.Printf("  [OK]   %s\n", name)
			return
		}
		fmt.Printf("  [SKIP] %s: %s\n", name, remedy)
		t.Skipf("%s: %s", name, remedy)
	}

	_, err := os.Stat(filepath.Join("/sys/fs/cgroup", "cgroup.controllers"))
	check("cgroup v2 unified hierarchy", err == nil,
		"mount cgroup v2 at /sys/fs/cgroup (not available: legacy cgroup v1 host?)")

	ctl, err := os.ReadFile(filepath.Join("/sys/fs/cgroup", "cgroup.controllers"))
	check("cpuset controller available", err == nil && strings.Contains(string(ctl), "cpuset"),
		"enable the cpuset controller in the kernel/cgroup config")

	check("running as root", os.Geteuid() == 0,
		"re-run with sudo: goset writes to /sys/fs/cgroup and steers irq affinity")

	topo, err := cpu.GetTopology()
	check("cpu topology readable", err == nil, fmt.Sprintf("read /sys/devices/system/cpu: %v", err))
	if err == nil {
		check(fmt.Sprintf("at least %d online cpu(s) (n + housekeeper)", minCPUs),
			topo.Online.Count() >= minCPUs,
			fmt.Sprintf("only %d online cpu(s) available, need %d", topo.Online.Count(), minCPUs))
	}
}
