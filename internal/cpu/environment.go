package cpu

import (
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
	env.SMT, _ = readFileTrim("/sys/devices/system/cpu/smt/control")
	env.Boost = readBoost()
	env.NumaBalancing, _ = readFileTrim("/proc/sys/kernel/numa_balancing")
	env.NmiWatchdog, _ = readFileTrim("/proc/sys/kernel/nmi_watchdog")
	env.THP, _ = readFileTrim("/sys/kernel/mm/transparent_hugepage/enabled")
	env.Mitigations = countMitigations()
	return env, nil
}


func readBoost() string {
	if text, err := readFileTrim("/sys/devices/system/cpu/cpufreq/boost"); err == nil {
		if text == "1" {
			return "on"
		}
		return "off"
	}
	if text, err := readFileTrim("/sys/devices/system/cpu/intel_pstate/no_turbo"); err == nil {
		if text == "0" {
			return "on"
		}
		return "off"
	}
	return "n/a"
}


func countMitigations() int {
	entries, err := os.ReadDir("/sys/devices/system/cpu/vulnerabilities")
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		text, err := readFileTrim(filepath.Join("/sys/devices/system/cpu/vulnerabilities", entry.Name()))
		if err != nil {
			continue
		}
		if !strings.HasPrefix(text, "Not affected") {
			count++
		}
	}
	return count
}
