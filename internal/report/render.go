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
}


func Render(w io.Writer, tables ...Table) {
	for _, tab := range tables {
		if tab.Title != "" {
			fmt.Fprintln(w, tab.Title)
		}
		widths := make([]int, len(tab.Header))
		for i, head := range tab.Header {
			widths[i] = len(head)
		}
		for _, row := range tab.Rows {
			for i, cell := range row {
				widths[i] = max(widths[i], len(cell))
			}
		}

		printRow(w, tab.Header, widths)
		for _, row := range tab.Rows {
			printRow(w, row, widths)
		}
		fmt.Fprintln(w)
	}
}


func printRow(w io.Writer, cells []string, widths []int) {
	var b strings.Builder
	b.WriteString("  ") // indent under title
	for i, cell := range cells {
		if i == 0 {
			fmt.Fprintf(&b, "%-*s", widths[i], cell)
		} else {
			fmt.Fprintf(&b, "  %*s", widths[i], cell)
		}
	}
	fmt.Fprintln(w, b.String())
}


func FormatValue(v float64) string {
	units := [...]struct {
		scale float64
		unit  string
	}{
		{1e12, "T"}, {1e9, "B"}, {1e6, "M"}, {1e3, "k"},
	}
	av := math.Abs(v)

	// lambda compression
	char := func(v float64) string {
		prec := 2
		switch av := math.Abs(v); {
		case av >= 100:
			prec = 0
		case av >= 10:
			prec = 1
		}
		s := strconv.FormatFloat(v, 'f', prec, 64)
		if len(s) > 4 {
			s = s[:4]
		}
		return s
	}

	for _, u := range units {
		if av >= u.scale {
			return char(v/u.scale) + u.unit
		}
	}
	return char(v)
}

func FormatTime(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.2fs", d.Seconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000)
	case d >= time.Microsecond:
		return fmt.Sprintf("%.2fus", float64(d.Nanoseconds())/1000)
	default:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
}
