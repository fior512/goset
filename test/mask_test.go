package integration

import (
	"testing"

	"goset/internal/cpu"
)

func TestParseCPUListRoundTrip(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"0", "0"},
		{"0,1,2", "0-2"},
		{"2,0,1", "0-2"},
		{"0-3,8", "0-3,8"},
		{"5-5", "5"},
		{"", ""},
	}
	for _, c := range cases {
		set, err := cpu.ParseCPUList(c.in)
		if err != nil {
			t.Fatalf("ParseCPUList(%q): %v", c.in, err)
		}
		if got := set.String(); got != c.want {
			t.Errorf("ParseCPUList(%q).String() = %q, want %q", c.in, got, c.want)
		}
	}
}


func TestParseCPUListInvalid(t *testing.T) {
	for _, in := range []string{"x", "1-", "-a"} {
		if _, err := cpu.ParseCPUList(in); err == nil {
			t.Errorf("ParseCPUList(%q): expected error, got nil", in)
		}
	}
}


func TestCPUSetAndAndNot(t *testing.T) {
	a, _ := cpu.ParseCPUList("0-3")
	b, _ := cpu.ParseCPUList("2-5")

	and := a
	and.And(b)
	if got := and.String(); got != "2-3" {
		t.Errorf("And = %q, want %q", got, "2-3")
	}

	andNot := a
	andNot.AndNot(b)
	if got := andNot.String(); got != "0-1" {
		t.Errorf("AndNot = %q, want %q", got, "0-1")
	}
}


func TestCPUSetCountAndSubset(t *testing.T) {
	full, _ := cpu.ParseCPUList("0-7")
	sub, _ := cpu.ParseCPUList("2,4")

	if got := full.Count(); got != 8 {
		t.Errorf("Count = %d, want 8", got)
	}
	if !sub.IsSubset(full) {
		t.Error("sub.IsSubset(full) = false, want true")
	}
	if full.IsSubset(sub) {
		t.Error("full.IsSubset(sub) = true, want false")
	}
}
