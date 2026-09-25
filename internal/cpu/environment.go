package cpu

import (
	"goset/internal/generic"
	"os"
	"path/filepath"
	"strings"
)

type Environment struct {
	SMT           *string // /sys/devices/system/cpu/smt/control, nil if unavailable
	Boost         string  // on/off/n/a, see ReadBoost
	NumaBalancing *string // /proc/sys/kernel/numa_balancing, nil if unavailable
	NmiWatchdog   *string // /proc/sys/kernel/nmi_watchdog, nil if unavailable
	THP           *string // /sys/kernel/mm/transparent_hugepage/enabled, nil if unavailable
	Mitigations   *int    // count of non-"Not affected" vulnerabilities, nil if unavailable
}

func GetEnvironment() (*Environment, error) {
	env := &Environment{}
	// https://www.kernel.org/doc/Documentation/ABI/testing/sysfs-devices-system-cpu
	env.SMT = readOptionalValue(generic.SysCPU + "/smt/control")
	env.Boost = ReadBoost(generic.SysCPU)
	// https://docs.kernel.org/admin-guide/sysctl/kernel.html
	env.NumaBalancing = readOptionalValue("/proc/sys/kernel/numa_balancing")
	env.NmiWatchdog = readOptionalValue("/proc/sys/kernel/nmi_watchdog")
	// https://docs.kernel.org/admin-guide/mm/transhuge.html
	env.THP = readOptionalValue("/sys/kernel/mm/transparent_hugepage/enabled")
	env.Mitigations = CountMitigations(generic.SysCPU + "/vulnerabilities")
	return env, nil
}

func readOptionalValue(path string) *string {
	text, err := readFileTrim(path)
	if err != nil || text == "" {
		return nil
	}
	return &text
}

// https://docs.kernel.org/admin-guide/pm/cpufreq.html
func ReadBoost(sysCPU string) string {
	if text, err := readFileTrim(sysCPU + "/cpufreq/boost"); err == nil && text != "" {
		if text == "1" {
			return "on"
		}
		return "off"
	}
	if text, err := readFileTrim(sysCPU + "/intel_pstate/no_turbo"); err == nil && text != "" {
		if text == "0" {
			return "on"
		}
		return "off"
	}
	return "n/a"
}

// https://www.kernel.org/doc/Documentation/ABI/testing/sysfs-devices-system-cpu
func CountMitigations(dir string) *int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	count := 0
	for _, entry := range entries {
		text, err := readFileTrim(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil
		}
		if !strings.HasPrefix(text, "Not affected") {
			count++
		}
	}
	return &count
}
