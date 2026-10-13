package isolation

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"goset/internal/generic"
)

// https://github.com/Irbalance/irqbalance
func (steering *Steering) holdIRQBalance() error {
	pid, err := ScanProcComm(generic.ProcRoot, generic.IRQBalanceComm)
	if err != nil {
		return err
	}
	managed := systemdActiveUnit(generic.IRQBalanceUnit)
	if pid == 0 && !managed {
		return nil
	}
	if managed {
		if err := exec.Command("systemctl", "stop", generic.IRQBalanceUnit).Run(); err == nil {
			steering.WasRunning = true
			steering.ViaSystemd = true
			steering.confirmStopped()
			return nil
		}
		if pid > 0 {
			fmt.Fprintf(os.Stderr, "%sirqbalance (pid %d): systemctl stop failed; steering may be overwritten\n", generic.LogPrefix, pid)
			steering.Warning = fmt.Sprintf("stop failed, still running pid %d", pid)
		} else {
			fmt.Fprintf(os.Stderr, "%sirqbalance: systemctl stop failed; steering may be overwritten\n", generic.LogPrefix)
			steering.Warning = "stop failed, still running, pid unknown"
		}
		steering.WasRunning = true
		return nil
	}
	fmt.Fprintf(os.Stderr, "%sirqbalance (pid %d) unmanaged; steering may be overwritten\n", generic.LogPrefix, pid)
	steering.WasRunning = true
	steering.Warning = fmt.Sprintf("unmanaged, still running pid %d", pid)
	return nil
}

// confirmStopped: a zero exit from systemctl is no proof the daemon is gone
func (steering *Steering) confirmStopped() {
	alive, err := ScanProcComm(generic.ProcRoot, generic.IRQBalanceComm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sirqbalance not re-checkable: %v\n", generic.LogPrefix, err)
		steering.Warning = "stop state unverified"
		return
	}
	if alive == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "%sirqbalance (pid %d) alive after stop; steering may be overwritten\n", generic.LogPrefix, alive)
	steering.Warning = fmt.Sprintf("still running pid %d", alive)
}

func (steering *Steering) Release() {
	if steering == nil || !steering.ViaSystemd {
		return
	}
	if err := exec.Command("systemctl", "start", generic.IRQBalanceUnit).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%sirqbalance restart failed: %v\n", generic.LogPrefix, err)
	}
}

func (steering *Steering) Status() string {
	switch {
	case steering == nil || !steering.WasRunning:
		return "absent"
	case steering.Warning != "":
		return steering.Warning
	default:
		return "held"
	}
}

// ScanProcComm returns the pid whose <root>/<pid>/comm equals name
func ScanProcComm(root, name string) (int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		comm, err := os.ReadFile(filepath.Join(root, entry.Name(), "comm"))
		if err != nil {
			continue // process exited mid-scan
		}
		if strings.TrimSpace(string(comm)) == name {
			return pid, nil
		}
	}
	return 0, nil
}

func systemdActiveUnit(unit string) bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	return exec.Command("systemctl", "is-active", "--quiet", unit).Run() == nil
}
