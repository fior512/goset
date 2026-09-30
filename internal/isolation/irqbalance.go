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
func (steer *Steering) holdIRQBalance() error {
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
			steer.WasRunning = true
			steer.ViaSystemd = true
			steer.confirmStopped()
			return nil
		}
		if pid > 0 {
			fmt.Fprintf(os.Stderr, "%sirqbalance (pid %d): systemctl stop failed; steering may be overwritten\n", generic.LogPrefix, pid)
			steer.Note = fmt.Sprintf("stop failed, still running pid %d", pid)
		} else {
			fmt.Fprintf(os.Stderr, "%sirqbalance: systemctl stop failed; steering may be overwritten\n", generic.LogPrefix)
			steer.Note = "stop failed, still running, pid unknown"
		}
		steer.WasRunning = true
		return nil
	}
	fmt.Fprintf(os.Stderr, "%sirqbalance (pid %d) unmanaged; steering may be overwritten\n", generic.LogPrefix, pid)
	steer.WasRunning = true
	steer.Note = fmt.Sprintf("unmanaged, still running pid %d", pid)
	return nil
}

// confirmStopped: a zero exit from systemctl is no proof the daemon is gone
func (steer *Steering) confirmStopped() {
	alive, err := ScanProcComm(generic.ProcRoot, generic.IRQBalanceComm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sirqbalance not re-checkable: %v\n", generic.LogPrefix, err)
		steer.Note = "stop state unverified"
		return
	}
	if alive == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "%sirqbalance (pid %d) alive after stop; steering may be overwritten\n", generic.LogPrefix, alive)
	steer.Note = fmt.Sprintf("still running pid %d", alive)
}

func (steer *Steering) Release() {
	if steer == nil || !steer.ViaSystemd {
		return
	}
	if err := exec.Command("systemctl", "start", generic.IRQBalanceUnit).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%sirqbalance restart failed: %v\n", generic.LogPrefix, err)
	}
}

func (steer *Steering) Status() string {
	switch {
	case steer == nil || !steer.WasRunning:
		return "absent"
	case steer.Note != "":
		return steer.Note
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
