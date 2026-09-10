package runner

import (
	"goset/internal/cli"
	"goset/internal/isolation"
)

func Run(cfg *cli.Config) error {
	return isolation.ApplyPin(cfg.Cmd)
}
