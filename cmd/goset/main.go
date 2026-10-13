package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"goset/internal/cli"
	"goset/internal/generic"
	"goset/internal/isolation"
	"goset/internal/runner"
	"goset/internal/version"
)

func main() {
	config, err := cli.Parse(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) { // usage already on stderr
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s%v\n", generic.LogPrefix, err)
		os.Exit(2)
	}

	if config.Version {
		fmt.Println(version.String())
		return
	}

	switch {
	case config.RmCgroup != "":
		err = isolation.RemoveCgroup(config.RmCgroup)
	case len(config.Task) == 0:
		err = runner.Diagnose()
	case len(config.Task) != 0:
		err = runner.Run(config)
	default:
		err = fmt.Errorf("flag error, no goset form defined")
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(runner.ExitCode(err))
		}
		fmt.Fprintf(os.Stderr, "%s%v\n", generic.LogPrefix, err)
		os.Exit(1)
	}
}
