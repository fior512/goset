package runner

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/generic"
	"goset/internal/isolation"
	"goset/internal/report"
	"goset/internal/telemetry"
)

func Telemetry(selected *generic.Selection, cfg *cli.Config, steering *isolation.Steering) (func(time.Duration, error, *isolation.Steering, *syscall.Rusage), error) {
	pinHousekeeper := func() error {
		var mask generic.CPUSet
		mask.SetBit(selected.HouseKeeper)
		return cpu.SetAffinity(0, mask)
	}

	sources := []telemetry.Source{
		&telemetry.IRQSource{Root: generic.ProcRoot},
		&telemetry.ThrottleSource{Root: generic.SysCPU},
		&telemetry.FreqSource{Root: generic.SysCPU},
		&telemetry.MigrationsSource{Root: generic.ProcRoot},
		&telemetry.RunqueueSource{Root: generic.ProcRoot},
	}
	if steering != nil {
		if expected := steering.ExpectedAffinities(); len(expected) > 0 {
			IRQDrift := &telemetry.IRQDriftSource{
				Root:     generic.ProcIRQ,
				Expected: expected,
				Drifted:  map[string]bool{},
			}

			sources = append(sources, IRQDrift)
		}
	}
	sampler := &telemetry.Sampler{
		Cpus:     selected.Task,
		Interval: time.Duration(cfg.SamplingMS) * time.Millisecond,
		Sources:  sources,
	}

	// HouseKeeper
	if err := sampler.Start(pinHousekeeper); err != nil {
		return nil, err
	}

	// lazy print
	lazyReport := func(wall time.Duration, runErr error, steering *isolation.Steering, rusage *syscall.Rusage) {
		fmt.Fprintln(os.Stderr, "\n\n----------------- GOSET -----------------")
		counters, polls, errs := sampler.Stop()
		rep := report.Report{
			Cpus:     selected.Task,
			Steer:    steering,
			Rusage:   rusage,
			Wall:     wall,
			Counters: counters,
			ExitCode: ExitCode(runErr),
			Polls:    polls,
			Interval: sampler.Interval,
			Errors:   errs,
		}
		tables := []report.Table{report.SelectionTable(selected)}
		tables = append(tables, report.TelemetryTables(rep, reportWidth(os.Stderr))...)
		tables = append(tables, report.RunTable(rep), report.ErrorsTable(rep))
		report.Render(os.Stderr, tables...)
	}
	return lazyReport, nil
}

// reportWidth is the terminal width of file, or 80 columns when file is not a terminal.
func reportWidth(file *os.File) int {
	var size struct{ rows, cols, xpixel, ypixel uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&size)))
	if errno != 0 || size.cols == 0 {
		return 80 // pipe or log file
	}
	return int(size.cols)
}
