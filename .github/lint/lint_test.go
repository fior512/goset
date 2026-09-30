package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// run parses one source file and returns the hits every check produces.
func run(t *testing.T, source string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "sample.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	found := newViolationList()
	for _, check := range []check{
		{"capitalized-local", capitalizedLocals},
		{"one-letter-local", oneLetterLocals},
		{"prose-comment", proseComments},
	} {
		check.run(file, fset, []byte(source), found)
	}
	sortViolations(found)
	hits := make([]string, 0, len(found.items))
	for _, item := range found.items {
		hits = append(hits, item.rule+": "+item.text)
	}
	return hits
}

func TestProseComments(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "a sentence closing a line is prose",
			source: "package p\n\n// The parser reads the file.\nfunc parse() {}\n",
			want:   []string{"prose-comment: terminal mark: The parser reads the file."},
		},
		{
			name:   "a doc comment narrating its identifier is prose",
			source: "package p\n\n// parse reads the file and trims it.\nfunc parse() {}\n",
			want:   []string{"prose-comment: narrated identifier: parse reads the file and trims it."},
		},
		{
			name:   "a trailing cue is not prose",
			source: "package p\n\nconst a = 1 // one per cpu, in order\n",
			want:   nil,
		},
		{
			name:   "a one word label is not prose",
			source: "package p\n\n// harvest\nfunc harvest() {}\n",
			want:   nil,
		},
		{
			name:   "a doc comment repeating only the name is not prose",
			source: "package p\n\n// parse\nfunc parse() {}\n",
			want:   nil,
		},
		{
			name:   "a path is a cue",
			source: "package p\n\n// read /sys/devices/system/cpu/online\nfunc read() {}\n",
			want:   nil,
		},
		{
			name:   "a url is a cue",
			source: "package p\n\n// https://man7.org/linux/man-pages/man2/sched_setaffinity.2.html\nfunc read() {}\n",
			want:   nil,
		},
		{
			name:   "a flag is a cue",
			source: "package p\n\n// -match overrides everything\nfunc read() {}\n",
			want:   nil,
		},
		{
			name:   "a version does not close a sentence",
			source: "package p\n\n// needs cgroup v2\nfunc read() {}\n",
			want:   nil,
		},
		{
			name:   "a directive is not prose",
			source: "package p\n\n//go:noinline\nfunc read() {}\n",
			want:   nil,
		},
		{
			name:   "prose inside a block comment is prose",
			source: "package p\n\n/*\n\tThen pick the top N element.\n*/\nfunc read() {}\n",
			want:   []string{"prose-comment: terminal mark: Then pick the top N element."},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assertHits(t, run(t, testCase.source), testCase.want)
		})
	}
}

func TestCapitalizedLocals(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "a define with a capital is prose shape, and a hit",
			source: "package p\n\nfunc run() {\n\tIndex := 1\n\t_ = Index\n}\n",
			want:   []string{"capitalized-local: Index"},
		},
		{
			name:   "a var declaration with a capital is a hit",
			source: "package p\n\nfunc run() {\n\tvar Count = 1\n\t_ = Count\n}\n",
			want:   []string{"capitalized-local: Count"},
		},
		{
			name:   "a range value with a capital is a hit",
			source: "package p\n\nfunc run(items []int) {\n\tfor _, Item := range items {\n\t\t_ = Item\n\t}\n}\n",
			want:   []string{"capitalized-local: Item"},
		},
		{
			name:   "a lowercase define is clean",
			source: "package p\n\nfunc run() {\n\tindex := 1\n\t_ = index\n}\n",
			want:   nil,
		},
		{
			name:   "an exported declaration is not a local",
			source: "package p\n\n// exported surface\nvar Index = 1\n",
			want:   nil,
		},
		{
			name:   "a struct field is not a local",
			source: "package p\n\ntype T struct{ Index int }\n\nfunc run() {\n\t_ = T{Index: 1}\n}\n",
			want:   nil,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assertHits(t, run(t, testCase.source), testCase.want)
		})
	}
}

func TestOneLetterLocals(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "a one letter define is a hit",
			source: "package p\n\nfunc run(text string) {\n\tb := len(text)\n\t_ = b\n}\n",
			want:   []string{"one-letter-local: b"},
		},
		{
			name:   "a three clause for index is exempt",
			source: "package p\n\nfunc run() {\n\tfor i := 0; i < 3; i++ {\n\t\t_ = i\n\t}\n}\n",
			want:   nil,
		},
		{
			name:   "a range index is exempt",
			source: "package p\n\nfunc run(items []int) {\n\tfor i := range items {\n\t\t_ = i\n\t}\n}\n",
			want:   nil,
		},
		{
			name:   "a range value is not exempt",
			source: "package p\n\nfunc run(items []int) {\n\tfor _, v := range items {\n\t\t_ = v\n\t}\n}\n",
			want:   []string{"one-letter-local: v"},
		},
		{
			name:   "a testing handle keeps its name",
			source: "package p\n\nimport \"testing\"\n\nfunc run(t *testing.T) {}\n",
			want:   nil,
		},
		{
			name:   "a blank identifier is not a name",
			source: "package p\n\nfunc run() {\n\tfor _ = range 3 {\n\t}\n}\n",
			want:   nil,
		},
		{
			name:   "a two letter name is clean",
			source: "package p\n\nfunc run() {\n\thi := 1\n\t_ = hi\n}\n",
			want:   nil,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assertHits(t, run(t, testCase.source), testCase.want)
		})
	}
}

func TestParseHunk(t *testing.T) {
	cases := []struct {
		header    string
		wantStart int
		wantCount int
	}{
		{"@@ -49 +49,2 @@ func run()", 49, 2},
		{"@@ -1,3 +1 @@ func run()", 1, 1},
		{"@@ -10,7 +12,9 @@", 12, 9},
		{"no hunk here", 0, 0},
		{"@@ garbage @@", 0, 0},
	}
	for _, testCase := range cases {
		start, count := parseHunk(testCase.header)
		if start != testCase.wantStart || count != testCase.wantCount {
			t.Errorf("parseHunk(%q) = %d,%d, want %d,%d",
				testCase.header, start, count, testCase.wantStart, testCase.wantCount)
		}
	}
}

func assertHits(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d hits %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Errorf("hit %d = %q, want prefix %q", i, got[i], want[i])
		}
	}
}
