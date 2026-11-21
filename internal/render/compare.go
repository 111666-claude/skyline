package render

import (
	"fmt"
	"html"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/pinkpixel-dev/skyline/internal/city"
)

// gapCols is the number of empty columns between the two cities.
const gapCols = 6

// CompareANSI draws two scenes side by side in the terminal: one title per
// city, the streets on the same baseline, and a single shared legend below.
// Titles longer than their city are shortened with an ellipsis. termWidth is
// the terminal width in columns (0 = unknown); when the pair doesn't fit, an
// error names both widths instead of printing a wrapped mess.
func CompareANSI(a, b *city.Scene, t Theme, titleA, titleB string, termWidth int) (string, error) {
	widthA, widthB := sceneCols(a), sceneCols(b)
	total := widthA + gapCols + widthB
	if termWidth > 0 && total > termWidth {
		return "", fmt.Errorf("comparison is %d columns wide but the terminal is %d; try -svg, a smaller -height, or shorter ranges", total, termWidth)
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(t.Roof))
	gap := lipgloss.NewStyle().Background(lipgloss.Color(t.Sky)).Render(strings.Repeat(" ", gapCols))

	var out strings.Builder
	out.WriteString(titleStyle.Render(truncateTitle(titleA, widthA)))
	out.WriteString(strings.Repeat(" ", total-widthB-lipgloss.Width(truncateTitle(titleA, widthA))))
	out.WriteString(titleStyle.Render(truncateTitle(titleB, widthB)))
	out.WriteString("\n\n")

	rowsA, rowsB := renderRows(a.Rows, t), renderRows(b.Rows, t)
	pad := lipgloss.NewStyle().Background(lipgloss.Color(t.Sky)).Render
	for i := 0; i < max(len(rowsA), len(rowsB)); i++ {
		var lineA, lineB string
		if i < len(rowsA) {
			lineA = rowsA[i]
		} else {
			lineA = pad(strings.Repeat(" ", widthA))
		}
		if i < len(rowsB) {
			lineB = rowsB[i]
		}
		out.WriteString(lineA + gap + lineB + "\n")
	}

	out.WriteByte('\n')
	out.WriteString(legendANSI(t))
	return out.String(), nil
}

// legendANSI is the shared window-level scale, drawn once for both cities.
func legendANSI(t Theme) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(t.Label)).Render("less "))
	for level := range 5 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(t.Windows[level])).
			Background(lipgloss.Color(t.Shades[0])).
			Render("▪"))
	}
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(t.Label)).Render(" more"))
	return b.String()
}

// CompareSVG draws the same side-by-side layout as CompareANSI as a
// standalone SVG. There is no width limit: the canvas simply grows.
func CompareSVG(a, b *city.Scene, t Theme, titleA, titleB string) string {
	colsA, colsB := sceneCols(a), sceneCols(b)
	rowsMax := max(len(a.Rows), len(b.Rows))
	gapPx := gapCols * cellW
	w := colsA*cellW + gapPx + colsB*cellW + padX*2
	h := rowsMax*cellH + padTop + padBot + legendH

	var out strings.Builder
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-labelledby="t">`, w, h, w, h)
	fmt.Fprintf(&out, `<title id="t">Skyline comparison of @%s: %s vs %s</title>`,
		html.EscapeString(a.Login), html.EscapeString(titleA), html.EscapeString(titleB))
	out.WriteString(svgStyle)
	fmt.Fprintf(&out, `<rect width="100%%" height="100%%" fill="%s" rx="8"/>`, t.Sky)

	fmt.Fprintf(&out, `<text x="%d" y="28" fill="%s" font-weight="700">%s</text>`,
		padX, t.Roof, html.EscapeString(truncateTitle(titleA, colsA)))
	fmt.Fprintf(&out, `<text x="%d" y="28" fill="%s" font-weight="700">%s</text>`,
		padX+colsA*cellW+gapPx, t.Roof, html.EscapeString(truncateTitle(titleB, colsB)))

	writeScene(&out, a, t, padX, padTop)
	writeScene(&out, b, t, padX+colsA*cellW+gapPx, padTop)

	// Shared legend: one "less ▪▪▪▪▪ more" row for both cities.
	y := padTop + rowsMax*cellH + legendH - 8
	fmt.Fprintf(&out, `<text x="%d" y="%d" fill="%s">less</text>`, padX, y, t.Label)
	for level := range 5 {
		fmt.Fprintf(&out, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`,
			padX+(5+level)*cellW, y-cellH+5, cellW, cellH, t.Shades[0])
		fmt.Fprintf(&out, `<text x="%d" y="%d" fill="%s">▪</text>`,
			padX+(5+level)*cellW, y, t.Windows[level])
	}
	fmt.Fprintf(&out, `<text x="%d" y="%d" fill="%s">more</text>`, padX+11*cellW, y, t.Label)

	out.WriteString(`</svg>`)
	return out.String()
}

// legendH is the vertical room the shared legend takes in the SVG.
const legendH = 28

// truncateTitle shortens a title to width cells, ending it with an ellipsis
// when anything was cut. Titles at most width cells long pass through.
func truncateTitle(title string, width int) string {
	runes := []rune(title)
	if len(runes) <= width {
		return title
	}
	if width < 2 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}
