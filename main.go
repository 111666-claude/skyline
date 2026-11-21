// Command skyline draws a GitHub or Gitea contribution graph as an ASCII city.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"

	"github.com/pinkpixel-dev/skyline/internal/city"
	"github.com/pinkpixel-dev/skyline/internal/gitea"
	"github.com/pinkpixel-dev/skyline/internal/github"
	"github.com/pinkpixel-dev/skyline/internal/render"
)

func main() {
	height := flag.Int("height", 14, "height of the tallest building, in rows")
	weeks := flag.Int("weeks", 53, "number of recent weeks to draw")
	svgPath := flag.String("svg", "", "write an animated SVG to this path instead of printing")
	themeName := flag.String("theme", render.Themes[0].Name, "color theme (see -themes)")
	listThemes := flag.Bool("themes", false, "list the built-in themes and exit")
	giteaURL := flag.String("gitea", "", "read contributions from the Gitea or Forgejo instance at this base URL")
	compare := flag.String("compare", "", `draw two ranges side by side: "2024-01-01..2024-12-31,2025-01-01..2025-12-31"`)
	titles := flag.String("titles", "", `titles for the two -compare cities, comma-separated (default: the ranges)`)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: skyline [flags] <username>")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *listThemes {
		lipgloss.Print(themeList())
		return
	}
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	var err error
	if *compare != "" {
		err = runCompare(flag.Arg(0), *giteaURL, *themeName, *height, *weeks, *svgPath, *compare, *titles)
	} else {
		err = run(flag.Arg(0), *giteaURL, *themeName, *height, *weeks, *svgPath)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "skyline:", err)
		os.Exit(1)
	}
}

func run(login, giteaURL, themeName string, height, weeks int, svgPath string) error {
	theme, err := render.ThemeByName(themeName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cal, err := fetch(ctx, login, giteaURL)
	if err != nil {
		return err
	}
	scene := city.Build(cal, city.Options{Height: height, Weeks: weeks})

	if svgPath == "" {
		lipgloss.Print(render.ANSI(scene, theme))
		return nil
	}
	if err := os.WriteFile(svgPath, []byte(render.SVG(scene, theme)), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "wrote", svgPath)
	return nil
}

// runCompare draws two date ranges as side-by-side cities on one height
// scale, with a title over each and a single shared legend.
func runCompare(login, giteaURL, themeName string, height, weeks int, svgPath, compare, titles string) error {
	ranges, err := parseCompare(compare, time.Now())
	if err != nil {
		return err
	}
	titleA, titleB, err := compareTitles(titles, ranges)
	if err != nil {
		return err
	}
	theme, err := render.ThemeByName(themeName)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	scenes := make([]*city.Scene, 2)
	cals := make([]*github.Calendar, 2)
	for i, r := range ranges {
		cal, err := fetchRange(ctx, login, giteaURL, r.from, r.to)
		if err != nil {
			return err
		}
		cals[i] = city.NormalizeRange(cal, r.from, r.to)
	}
	scenes[0], scenes[1] = city.BuildCompare(cals[0], cals[1], city.Options{Height: height, Weeks: weeks})

	if svgPath == "" {
		width, _, err := term.GetSize(os.Stdout.Fd())
		if err != nil {
			width = 0 // not a terminal: skip the fit check
		}
		out, err := render.CompareANSI(scenes[0], scenes[1], theme, titleA, titleB, width)
		if err != nil {
			return err
		}
		lipgloss.Print(out)
		return nil
	}
	out := render.CompareSVG(scenes[0], scenes[1], theme, titleA, titleB)
	if err := os.WriteFile(svgPath, []byte(out), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "wrote", svgPath)
	return nil
}

// dateRange is one half of a comparison, inclusive on both ends.
type dateRange struct {
	from, to time.Time
}

// parseCompare parses "YYYY-MM-DD..YYYY-MM-DD,YYYY-MM-DD..YYYY-MM-DD".
// Rules: a range whose start is after its end is an error; the two ranges
// must not overlap; an end date in the future is clamped to today, and a
// range that starts in the future is empty and therefore an error.
func parseCompare(s string, now time.Time) ([2]dateRange, error) {
	var out [2]dateRange
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return out, fmt.Errorf("compare: want two ranges like 2024-01-01..2024-12-31,2025-01-01..2025-12-31, got %q", s)
	}
	today := now.UTC().Truncate(24 * time.Hour)
	for i, part := range parts {
		bounds := strings.Split(strings.TrimSpace(part), "..")
		if len(bounds) != 2 {
			return out, fmt.Errorf("compare: bad range %q, want FROM..TO", part)
		}
		from, err := time.Parse("2006-01-02", strings.TrimSpace(bounds[0]))
		if err != nil {
			return out, fmt.Errorf("compare: bad start date in %q: %v", part, err)
		}
		to, err := time.Parse("2006-01-02", strings.TrimSpace(bounds[1]))
		if err != nil {
			return out, fmt.Errorf("compare: bad end date in %q: %v", part, err)
		}
		if from.After(to) {
			return out, fmt.Errorf("compare: empty range %q (start is after end)", part)
		}
		if to.After(today) {
			to = today
		}
		if from.After(today) {
			return out, fmt.Errorf("compare: empty range %q (it lies entirely in the future)", part)
		}
		out[i] = dateRange{from, to}
	}
	if !out[0].to.Before(out[1].from) && !out[1].to.Before(out[0].from) {
		return out, fmt.Errorf("compare: ranges %q and %q overlap", parts[0], parts[1])
	}
	return out, nil
}

// compareTitles resolves the two city titles: the -titles flag when given,
// otherwise the date ranges themselves.
func compareTitles(flagValue string, ranges [2]dateRange) (string, string, error) {
	format := func(r dateRange) string {
		return r.from.Format("2006-01-02") + " → " + r.to.Format("2006-01-02")
	}
	if flagValue == "" {
		return format(ranges[0]), format(ranges[1]), nil
	}
	parts := strings.SplitN(flagValue, ",", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("titles: want two non-empty titles separated by a comma, got %q", flagValue)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

// fetch reads the calendar from Gitea when a base URL is given, otherwise GitHub.
func fetch(ctx context.Context, login, giteaURL string) (*github.Calendar, error) {
	if giteaURL != "" {
		return gitea.Fetch(ctx, giteaURL, gitea.Token(), login)
	}
	token, err := github.Token()
	if err != nil {
		return nil, err
	}
	return github.Fetch(ctx, token, login)
}

// fetchRange reads one date range from Gitea or GitHub, like fetch.
func fetchRange(ctx context.Context, login, giteaURL string, from, to time.Time) (*github.Calendar, error) {
	if giteaURL != "" {
		return gitea.FetchRange(ctx, giteaURL, gitea.Token(), login, from, to)
	}
	token, err := github.Token()
	if err != nil {
		return nil, err
	}
	return github.FetchRange(ctx, token, login, from, to)
}

// themeList prints each theme name next to a swatch of its roof and window colors.
func themeList() string {
	var b strings.Builder
	for _, t := range render.Themes {
		name := lipgloss.NewStyle().Width(11).Foreground(lipgloss.Color(t.Text)).Render(t.Name)
		swatch := lipgloss.NewStyle().Background(lipgloss.Color(t.Sky)).Foreground(lipgloss.Color(t.Roof)).Render(" ▁▁ ")
		for _, c := range t.Windows {
			swatch += lipgloss.NewStyle().Background(lipgloss.Color(t.Shades[0])).Foreground(lipgloss.Color(c)).Render("▪▪")
		}
		b.WriteString(name + swatch + "\n")
	}
	return b.String()
}
