package main

import (
	"flag"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"goset/benchmark/plugin"
)

// numField is the strict numeric-field grammar: optional sign, digits with
// at most one dot, optional exponent, optional unit suffix starting with a
// Unicode letter or percent sign. A field that does not match entirely is
// a label; fields are never partially parsed ("1,234", "1.2.3", "10-20"
// become labels).
var numField = regexp.MustCompile(`^([+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?)([\p{L}%][\p{L}0-9%/_-]*)?$`)

// maxExtractedKeys bounds degenerate outputs (interleaved logs): keys with
// the most samples survive.
const maxExtractedKeys = 32

// externalCmd adapts an arbitrary command passed after "--" to the
// plugin.Benchmark interface. Protocol: none. Every numeric field of the
// task's output is keyed by the line's masked label (numerics replaced by
// N), the field's column ordinal, and the unit bound to it, so
// per-iteration table rows group into one series. Runs are compared by
// key, never by line position. Values are raw: no unit is ever scaled, so
// magnitudes of different units never mix inside one series. A task with
// no extractable numbers falls back to wall(s).
type externalCmd struct {
	argv []string
	last map[string]int // output-shape fingerprint of the last Metrics call
}

func (e *externalCmd) Name() string { return "external" }

func (e *externalCmd) RegisterFlags(fs *flag.FlagSet) {}

func (e *externalCmd) Build() (string, error) { return e.argv[0], nil }

func (e *externalCmd) Argv(string) []string { return e.argv }

// fingerprint is the last run's output structure: how often each line
// shape occurred. main.go compares fingerprints across runs of one mode
// and between modes to detect output drift.
func (e *externalCmd) fingerprint() map[string]int { return e.last }

// parsedLine is one output line classified for extraction. mask is the
// alignment label (numerics replaced by N, bound units removed), values
// are raw parsed floats, units are bound or attached unit strings,
// displays are compact labels, cols are source column ordinals.
type parsedLine struct {
	mask     []string
	values   []float64
	units    []string
	displays []string
	cols     []int
}

type series struct {
	name    string // alignment identity
	display string // compact display label
	col     int    // source column ordinal
	samples []float64
}

func (e *externalCmd) Metrics(output []byte) []plugin.Metric {
	seriesByName := map[string]*series{}
	var names []string
	e.last = map[string]int{}
	for _, line := range strings.Split(string(output), "\n") {
		if i := strings.LastIndexByte(line, '\r'); i >= 0 {
			line = line[i+1:] // progress lines: last segment wins
		}
		parsed := parseLine(line)
		if len(parsed.values) == 0 {
			continue
		}
		mask := strings.Join(parsed.mask, " ")
		e.last[shapeOf(mask, parsed.units)]++
		for i, value := range parsed.values {
			name := seriesName(mask, parsed.cols[i], parsed.units[i])
			s := seriesByName[name]
			if s == nil {
				s = &series{name: name, display: parsed.displays[i], col: parsed.cols[i]}
				if s.display == "" {
					s.display = name
				}
				seriesByName[name] = s
				names = append(names, name)
			}
			s.samples = append(s.samples, value)
		}
	}
	if len(names) > maxExtractedKeys {
		sort.SliceStable(names, func(i, j int) bool {
			return len(seriesByName[names[i]].samples) > len(seriesByName[names[j]].samples)
		})
		names = names[:maxExtractedKeys]
	}
	metrics := make([]plugin.Metric, 0, len(names))
	for _, name := range names {
		s := seriesByName[name]
		sorted := append([]float64(nil), s.samples...)
		sort.Float64s(sorted)
		metrics = append(metrics, plugin.Metric{
			Name:    s.name,
			Display: s.display,
			Value:   sorted[len(sorted)/2],
			Samples: s.samples,
			Col:     s.col,
		})
	}
	return metrics
}

// parseLine splits a line into fields and classifies each as numeric or
// label. One binding rule: the label immediately following a numeric
// binds to it as its unit, trailing or between two numerics, when the
// numeric carries no attached suffix of its own. Purely positional, no
// unit list. The display label is the leading row name plus the bound
// unit, empty when the line carries neither.
func parseLine(line string) parsedLine {
	fields := strings.Fields(line)
	numeric := make([]bool, len(fields))
	values := make([]float64, len(fields))
	units := make([]string, len(fields))
	for i, field := range fields {
		if v, u, ok := parseNumField(field); ok {
			numeric[i] = true
			values[i] = v
			units[i] = u
		}
	}
	bound := make([]bool, len(fields))
	for i := 1; i < len(fields); i++ {
		if !numeric[i] && numeric[i-1] && units[i-1] == "" {
			units[i-1] = fields[i]
			bound[i] = true
		}
	}
	var parsed parsedLine
	var rowName []string
	seenNumber := false
	col := 0
	for i, field := range fields {
		switch {
		case numeric[i]:
			seenNumber = true
			parsed.mask = append(parsed.mask, "N")
			parsed.values = append(parsed.values, values[i])
			parsed.units = append(parsed.units, units[i])
			parsed.cols = append(parsed.cols, col)
			parsed.displays = append(parsed.displays, displayLabel(rowName, units[i]))
			col++
		case bound[i]:
		case !seenNumber:
			rowName = append(rowName, field)
			parsed.mask = append(parsed.mask, field)
		default:
			parsed.mask = append(parsed.mask, field)
		}
	}
	return parsed
}

// displayLabel is the row name followed by the bound unit, the part a
// reader needs to tell one column of a multi-column row from another.
func displayLabel(rowName []string, unit string) string {
	parts := append([]string{}, rowName...)
	if unit != "" {
		parts = append(parts, unit)
	}
	return strings.Join(parts, " ")
}

// seriesName is the alignment identity: masked line, column ordinal and
// bound unit. The unit belongs to the identity so raw values carrying
// different units ("831ns" vs "0.83us") never share one series.
func seriesName(mask string, col int, unit string) string {
	if unit == "" {
		return fmt.Sprintf("%s #%d", mask, col)
	}
	return fmt.Sprintf("%s #%d %s", mask, col, unit)
}

// shapeOf is a line's output-structure fingerprint: the masked line plus
// its unit tuple, so a unit flip between runs counts as drift.
func shapeOf(mask string, units []string) string {
	return mask + " [" + strings.Join(units, " ") + "]"
}

// parseNumField parses one field against the strict grammar. Returns the
// raw value, never scaled, and the raw attached unit suffix. Normalizing
// units here would merge incomparable magnitudes; the unit stays opaque
// and reaches the report as display text.
func parseNumField(field string) (float64, string, bool) {
	m := numField.FindStringSubmatch(field)
	if m == nil {
		return 0, "", false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, "", false
	}
	return v, m[2], true
}
