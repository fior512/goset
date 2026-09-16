package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"goset/internal/cli"
	"goset/internal/isolation"
	"goset/internal/runner"
)

func main() {
	cfg, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "goset:", err)
		os.Exit(2)
	}

	switch {
	case cfg.RmCgroup != "":
		err = isolation.RemoveCgroup(cfg.RmCgroup)
	case len(cfg.Task) == 0:
		err = runner.Diagnose(cfg)
	default:
		err = runner.Run(cfg)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "goset:", err)
		os.Exit(1)
	}
}
