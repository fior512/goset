package runner

import (
	"os"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/isolation"
	"goset/internal/report"
)

// used in bare BASH("goset")
func Diagnose(cfg *cli.Config) error {
	topo, err := cpu.GetTopology()
	if err != nil {
		return err
	}
	cgroups, err := isolation.ListCgroups()
	if err != nil {
		return err
	}
	env, err := cpu.GetEnvironment()
	if err != nil {
		return err
	}
	report.Render(os.Stdout,
		report.TopologyTable(topo),
		report.EnvironmentTable(env),
		report.CgroupTable(cgroups))
	return nil
}
