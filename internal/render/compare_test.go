package render

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/pinkpixel-dev/skyline/internal/city"
	"github.com/pinkpixel-dev/skyline/internal/github"
)

// pair builds two small scenes on a shared scale from a fixed calendar, so
// the expected layout is fully deterministic.
func pair() (a, b *city.Scene) {
	mk := func(start string, weeks [][]int) *github.Calendar {
		d, _ := time.Parse("2006-01-02", start)
		cal := &github.Calendar{Login: "octo"}
		for _, counts := range weeks {
			var week []github.Day
			for _, n := range counts {
				week = append(week, github.Day{Date: d, Count: n, Level: min(4, n)})
				cal.Total += n
				d = d.AddDate(0, 0, 1)
			}
			cal.Weeks = append(cal.Weeks, week)
		}
		return cal
	}
	return city.BuildCompare(
		mk("2025-01-05", [][]int{
			{4, 0, 4, 0, 4, 0, 4}, {0, 9, 0, 9, 0, 9, 0}, {2, 2, 0, 0, 2, 2, 0},
			{7, 0, 0, 0, 0, 0, 7}, {0, 0, 3, 3, 0, 0, 0}, {5, 5, 5, 0, 0, 5, 5},
		}),
		mk("2026-01-04", [][]int{
			{9, 9, 0, 0, 9, 9, 0}, {1, 1, 1, 1, 1, 1, 1}, {0, 4, 0, 4, 0, 4, 0},
			{6, 0, 6, 0, 6, 0, 6}, {3, 3, 3, 3, 0, 0, 0}, {8, 0, 0, 8, 0, 0, 8},
		}),
		city.Options{Height: 6},
	)
}

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;:]*[a-zA-Z]")

func visible(t *testing.T, s string) string {
	t.Helper()
	return ansiEscape.ReplaceAllString(s, "")
}

func TestCompareANSILayout(t *testing.T) {
	a, b := pair()
	out, err := CompareANSI(a, b, Themes[0], "last year", "this year", 0)
	if err != nil {
		t.Fatal(err)
	}
	plain := visible(t, out)

	if !strings.Contains(plain, "last year") || !strings.Contains(plain, "this year") {
		t.Error("output should carry both city titles")
	}
	if n := strings.Count(plain, "less"); n != 1 {
		t.Errorf("legend appears %d times, want exactly 1 shared legend", n)
	}
	if !strings.Contains(plain, "more") {
		t.Error("legend is missing its upper bound")
	}

	// The second title starts exactly above the second city, and every city
	// row is exactly as wide as the two cities plus the gap, so the streets
	// line up. The legend underneath is shared and only as wide as it needs
	// to be.
	want := sceneCols(a) + gapCols + sceneCols(b)
	lines := strings.Split(strings.TrimRight(plain, "\n"), "\n")
	if got := strings.Index(lines[0], "this year"); got != sceneCols(a)+gapCols {
		t.Errorf("second title starts at column %d, want %d", got, sceneCols(a)+gapCols)
	}
	for i, line := range lines[2 : 2+len(a.Rows)] {
		if w := lipgloss.Width(line); w != want {
			t.Errorf("city row %d is %d cells wide, want %d: %q", i, w, want, line)
		}
	}
}

func TestCompareANSITooNarrow(t *testing.T) {
	a, b := pair()
	width := sceneCols(a) + gapCols + sceneCols(b)

	if _, err := CompareANSI(a, b, Themes[0], "a", "b", width); err != nil {
		t.Errorf("exact fit should print, got %v", err)
	}
	_, err := CompareANSI(a, b, Themes[0], "a", "b", width-1)
	if err == nil {
		t.Fatal("narrow terminal should be an error, not wrapped output")
	}
	msg := err.Error()
	if !strings.Contains(msg, fmt.Sprint(width)) || !strings.Contains(msg, fmt.Sprint(width-1)) {
		t.Errorf("error should name both widths, got %q", msg)
	}
}

func TestCompareTitlesAreTruncated(t *testing.T) {
	a, b := pair()
	long := strings.Repeat("y", 200)
	out, err := CompareANSI(a, b, Themes[0], long, "b", 0)
	if err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(visible(t, out), "\n", 2)[0]
	if !strings.Contains(first, "…") {
		t.Error("overlong title should end in an ellipsis")
	}
	if strings.Contains(first, long) {
		t.Error("overlong title was not truncated")
	}
}

func TestTruncateTitle(t *testing.T) {
	if got := truncateTitle("short", 10); got != "short" {
		t.Errorf("short title = %q, want untouched", got)
	}
	if got := truncateTitle("abcdefghij", 5); got != "abcd…" {
		t.Errorf("truncated = %q, want %q", got, "abcd…")
	}
	if got := truncateTitle("abc", 1); got != "…" {
		t.Errorf("tiny width = %q, want just an ellipsis", got)
	}
}

func TestCompareSVGMatchesANSI(t *testing.T) {
	a, b := pair()
	theme := Themes[0]
	ansiOut, err := CompareANSI(a, b, theme, "last year", "this year", 0)
	if err != nil {
		t.Fatal(err)
	}
	svg := CompareSVG(a, b, theme, "last year", "this year")

	// Well-formed XML.
	dec := xml.NewDecoder(strings.NewReader(svg))
	for {
		if _, err := dec.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("invalid XML: %v", err)
		}
	}

	// The canvas is exactly the two cities plus the gap and padding.
	colsA, colsB := sceneCols(a), sceneCols(b)
	wantW := colsA*cellW + gapCols*cellW + colsB*cellW + padX*2
	if !strings.Contains(svg, fmt.Sprintf(`width="%d"`, wantW)) {
		t.Errorf("SVG width is not %d (two cities + gap + padding)", wantW)
	}

	for _, want := range []string{"last year", "this year", "less", "more", "prefers-reduced-motion"} {
		if !strings.Contains(svg, want) {
			t.Errorf("SVG missing %q", want)
		}
	}

	// SVG and ANSI draw the same city: every lit window (▪) and dark window
	// (·) in the terminal output must show up in the SVG, legend included.
	plain := visible(t, ansiOut)
	for _, r := range []rune{'▪', '·', '▁', '━'} {
		if na, ns := strings.Count(plain, string(r)), strings.Count(svg, string(r)); na != ns {
			t.Errorf("glyph %q appears %d times in ANSI but %d in SVG", r, na, ns)
		}
	}
}

func TestCompareSVGEscapesTitles(t *testing.T) {
	a, b := pair()
	svg := CompareSVG(a, b, Themes[0], `<script>`, `a & b`)
	if strings.Contains(svg, "<script>") {
		t.Error("title must be escaped")
	}
	if !strings.Contains(svg, "a &amp; b") {
		t.Error("ampersand in title must be escaped")
	}
}
