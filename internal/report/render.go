package report

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

type Table struct {
	Title  string
	Header []string
	Rows   [][]string
	Align  Align
}

// Align picks the left-aligned columns; every other column is right-aligned.
type Align int

const (
	AlignLabel Align = iota // column 0 only
	AlignLeft               // every column
	AlignRight              // none
)


func Render(out io.Writer, tables ...Table) {
	for _, tab := range tables {
		if len(tab.Rows) == 0 {
			continue
		}
		if tab.Title != "" {
			fmt.Fprintln(out, tab.Title)
		}
		widths := columnWidths(tab)
		if len(tab.Header) > 0 {
			printRow(out, tab.Header, widths, tab.Align)
		}
		for _, row := range tab.Rows {
			printRow(out, row, widths, tab.Align)
		}
		fmt.Fprintln(out)
	}
}


func columnWidths(tab Table) []int {
	widths := make([]int, 0, len(tab.Header))
	widest := func(row []string) {
		for len(widths) < len(row) {
			widths = append(widths, 0)
		}
		for i, cell := range row {
			widths[i] = max(widths[i], len(cell))
		}
	}
	widest(tab.Header)
	for _, row := range tab.Rows {
		widest(row)
	}
	return widths
}

const gutter = 2

// lineWidth is the printed length of a row whose columns have these widths.
func lineWidth(widths []int) int {
	total := gutter
	for i, width := range widths {
		if i > 0 {
			total += gutter
		}
		total += width
	}
	return total
}


func printRow(out io.Writer, cells []string, widths []int, align Align) {
	var buf strings.Builder
	buf.WriteString(strings.Repeat(" ", gutter))
	for i, cell := range cells {
		if i > 0 {
			buf.WriteString(strings.Repeat(" ", gutter))
		}
		left := align == AlignLeft || (align == AlignLabel && i == 0)
		if left {
			fmt.Fprintf(&buf, "%-*s", widths[i], cell)
		} else {
			fmt.Fprintf(&buf, "%*s", widths[i], cell)
		}
	}
	fmt.Fprintln(out, strings.TrimRight(buf.String(), " "))
}


func compress(value float64) string {
	prec := 2
	switch av := math.Abs(value); {
	case value == math.Trunc(value):
		prec = 0
	case av >= 100:
		prec = 0
	case av >= 10:
		prec = 1
	}
	for {
		str := strconv.FormatFloat(value, 'f', prec, 64)
		if len(str) <= 4 || prec == 0 {
			return str
		}
		prec--
	}
}


func FormatValue(value float64) string {
	units := [...]struct {
		scale float64
		unit  string
	}{
		{1e12, "T"}, {1e9, "B"}, {1e6, "M"}, {1e3, "k"},
	}
	av := math.Abs(value)
	for _, unit := range units {
		if av >= unit.scale {
			return compress(value/unit.scale) + unit.unit
		}
	}
	return compress(value)
}


// FormatFreq renders a Hz value with SI prefixes (k/M/G/T), not the
// count-style k/M/B/T used by FormatValue.
func FormatFreq(hz float64) string {
	units := [...]struct {
		scale float64
		unit  string
	}{
		{1e12, "T"}, {1e9, "G"}, {1e6, "M"}, {1e3, "k"},
	}
	av := math.Abs(hz)
	for _, unit := range units {
		if av >= unit.scale {
			return compress(hz/unit.scale) + unit.unit + "Hz"
		}
	}
	return compress(hz) + "Hz"
}


func FormatTime(elapsed time.Duration) string {
	switch {
	case elapsed >= time.Second:
		return fmt.Sprintf("%.2fs", elapsed.Seconds())
	case elapsed >= time.Millisecond:
		return fmt.Sprintf("%.2fms", float64(elapsed.Microseconds())/1000)
	case elapsed >= time.Microsecond:
		return fmt.Sprintf("%.2fus", float64(elapsed.Nanoseconds())/1000)
	default:
		return fmt.Sprintf("%dns", elapsed.Nanoseconds())
	}
}
