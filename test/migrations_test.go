package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"goset/internal/generic"
	"goset/internal/telemetry"
)

func writeSched(t *testing.T, root string, pid, tid int, migrations int) {
	t.Helper()
	dir := filepath.Join(root, strconv.Itoa(pid), "task", strconv.Itoa(tid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("se.vruntime                              :        100.0\nse.nr_migrations                             : %8d\nnr_switches                                  : %8d\n", migrations, migrations*10)
	if err := os.WriteFile(filepath.Join(dir, "sched"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeChildren(t *testing.T, root string, childPIDs ...int) {
	t.Helper()
	self := strconv.Itoa(os.Getpid())
	dir := filepath.Join(root, self, "task", self)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var fields string
	for i, pid := range childPIDs {
		if i > 0 {
			fields += " "
		}
		fields += strconv.Itoa(pid)
	}
	if err := os.WriteFile(filepath.Join(dir, "children"), []byte(fields+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func migrationValue(t *testing.T, src *telemetry.MigrationsSource) (int, bool) {
	t.Helper()
	return telemetry.CountMigrations(src.Summary())
}

func TestMigrationsSumsTaskThreads(t *testing.T) {
	root := t.TempDir()
	const child = 4242
	writeChildren(t, root, child)
	writeSched(t, root, child, child, 3)
	writeSched(t, root, child, child+1, 5)

	src := &telemetry.MigrationsSource{Root: root}
	if err := src.Baseline(generic.CPUSet{}); err != nil {
		t.Fatal(err)
	}
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	got, ok := migrationValue(t, src)
	if !ok {
		t.Fatal("Summary reported no migration counter after a live child sample")
	}
	if got != 8 {
		t.Errorf("migrations = %d, want 8 (3+5 over task threads)", got)
	}
}

func TestMigrationsStickyAfterChildVanishes(t *testing.T) {
	root := t.TempDir()
	const child = 4242
	writeChildren(t, root, child)
	writeSched(t, root, child, child, 7)

	src := &telemetry.MigrationsSource{Root: root}
	if err := src.Baseline(generic.CPUSet{}); err != nil {
		t.Fatal(err)
	}
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	if got, _ := migrationValue(t, src); got != 7 {
		t.Fatalf("migrations = %d, want 7", got)
	}

	// child reaped: children entry and sched gone, Stop keeps last value
	if err := os.RemoveAll(filepath.Join(root, strconv.Itoa(child))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, strconv.Itoa(os.Getpid()), "task", strconv.Itoa(os.Getpid()), "children"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}
	got, ok := migrationValue(t, src)
	if !ok {
		t.Fatal("Summary dropped the migration counter after child vanished")
	}
	if got != 7 {
		t.Errorf("migrations = %d after reap, want sticky 7", got)
	}
}

func TestMigrationsNoChildNoCounter(t *testing.T) {
	src := &telemetry.MigrationsSource{Root: t.TempDir()}
	if err := src.Baseline(generic.CPUSet{}); err != nil {
		t.Fatal(err)
	}
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	if err := src.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, ok := migrationValue(t, src); ok {
		t.Error("Summary reported a migration counter with no child ever sampled")
	}
}

func TestMigrationsMissingSchedFile(t *testing.T) {
	root := t.TempDir()
	const child = 4242
	writeChildren(t, root, child) // child listed but sched unreadable (mid-exit race)

	src := &telemetry.MigrationsSource{Root: root}
	if err := src.Baseline(generic.CPUSet{}); err != nil {
		t.Fatal(err)
	}
	if err := src.Poll(); err != nil {
		t.Fatal(err)
	}
	if _, ok := migrationValue(t, src); ok {
		t.Error("Summary reported a migration counter when sched was unreadable")
	}
}
