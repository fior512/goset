package cpu

import (
	"goset/internal/generic"
	"os"
	"path/filepath"
	"strings"
)

type Environment struct {
	SMT           string // /sys/devices/system/cpu/smt/control
	Boost         string // on/off/n/a, see readBoost
	NumaBalancing string // /proc/sys/kernel/numa_balancing
	NmiWatchdog   string // /proc/sys/kernel/nmi_watchdog
	THP           string // /sys/kernel/mm/transparent_hugepage/enabled
	Mitigations   int    // count of non-"Not affected" vulnerabilities
}

func GetEnvironment() (*Environment, error) {
	env := &Environment{}
	// https://www.kernel.org/doc/Documentation/ABI/testing/sysfs-devices-system-cpu
	env.SMT, _ = readFileTrim(generic.SysCPU + "/smt/control")
	env.Boost = readBoost()
	// https://docs.kernel.org/admin-guide/sysctl/kernel.html
	env.NumaBalancing, _ = readFileTrim("/proc/sys/kernel/numa_balancing")
	env.NmiWatchdog, _ = readFileTrim("/proc/sys/kernel/nmi_watchdog")
	// https://docs.kernel.org/admin-guide/mm/transhuge.html
	env.THP, _ = readFileTrim("/sys/kernel/mm/transparent_hugepage/enabled")
	env.Mitigations = countMitigations()
	return env, nil
}

// https://docs.kernel.org/admin-guide/pm/cpufreq.html
func readBoost() string {
	if text, err := readFileTrim(generic.SysCPU + "/cpufreq/boost"); err == nil {
		if text == "1" {
			return "on"
		}
		return "off"
	}
	if text, err := readFileTrim(generic.SysCPU + "/intel_pstate/no_turbo"); err == nil {
		if text == "0" {
			return "on"
		}
		return "off"
	}
	return "n/a"
}

// https://www.kernel.org/doc/Documentation/ABI/testing/sysfs-devices-system-cpu
func countMitigations() int {
	entries, err := os.ReadDir(generic.SysCPU + "/vulnerabilities")
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		text, err := readFileTrim(filepath.Join(generic.SysCPU+"/vulnerabilities", entry.Name()))
		if err != nil {
			continue
		}
		if !strings.HasPrefix(text, "Not affected") {
			count++
		}
	}
	return count
}
