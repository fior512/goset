package runner

import (
	"fmt"
	"os"

	"goset/internal/cli"
	"goset/internal/cpu"
	"goset/internal/isolation"
	"goset/internal/report"
)

// used in bare goset
func Diagnose(cfg *cli.Config) error {
	topo, err := cpu.GetTopology()
	if err != nil {
		return fmt.Errorf("read topology: %w", err)
	}
	cgroups, err := isolation.ListCgroups()
	if err != nil {
		return fmt.Errorf("list cgroups: %w", err)
	}
	env, err := cpu.GetEnvironment()
	if err != nil {
		return fmt.Errorf("read environment: %w", err)
	}
	report.Render(os.Stdout,
		report.TopologyTable(topo),
		report.EnvironmentTable(env),
		report.CgroupTable(cgroups))
	return nil
}
