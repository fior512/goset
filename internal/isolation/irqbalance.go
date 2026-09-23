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

const irqBalanceUnit = "irqbalance.service"


// https://github.com/Irbalance/irqbalance
func (steer *Steering) holdIRQBalance() error {
	pid, err := ScanProcComm("/proc", "irqbalance")
	if err != nil {
		return err
	}
	if pid == 0 {
		return nil
	}
	if systemdActiveUnit(irqBalanceUnit) {
		if err := exec.Command("systemctl", "stop", irqBalanceUnit).Run(); err == nil {
			steer.WasRunning = true
			steer.ViaSystemd = true
			return nil
		}
		fmt.Fprintf(os.Stderr, "%sirqbalance (pid %d): systemctl stop failed; steering may be overwritten\n", generic.LogPrefix, pid)
		steer.WasRunning = true
		steer.Note = fmt.Sprintf("stop failed pid %d", pid)
		return nil
	}
	fmt.Fprintf(os.Stderr, "%sirqbalance (pid %d) unmanaged; steering may be overwritten\n", generic.LogPrefix, pid)
	steer.WasRunning = true
	steer.Note = fmt.Sprintf("unmanaged pid %d", pid)
	return nil
}

func (steer *Steering) Release() {
	if steer == nil || !steer.ViaSystemd {
		return
	}
	if err := exec.Command("systemctl", "start", irqBalanceUnit).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%sirqbalance restart failed: %v\n", generic.LogPrefix, err)
	}
}

func (steer *Steering) Status() string {
	switch {
	case steer == nil || !steer.WasRunning:
		return "no"
	case steer.Note != "":
		return steer.Note
	default:
		return "yes"
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
