package city

import (
	"testing"
	"time"

	"github.com/pinkpixel-dev/skyline/internal/github"
)

// sparseCalendar builds a calendar from a date→count map, like a source that
// only returns the days it knows about.
func sparseCalendar(days map[string]int) *github.Calendar {
	cal := &github.Calendar{Login: "octo"}
	var week []github.Day
	for date, n := range days {
		d, _ := time.Parse("2006-01-02", date)
		week = append(week, github.Day{Date: d, Count: n, Level: min(4, n)})
	}
	if len(week) > 0 {
		cal.Weeks = [][]github.Day{week}
	}
	return cal
}

func weekDates(t *testing.T, cal *github.Calendar, week int) []string {
	t.Helper()
	var out []string
	for _, d := range cal.Weeks[week] {
		out = append(out, d.Date.Format("2006-01-02"))
	}
	return out
}

func TestNormalizeRangeExpandsToFullWeeks(t *testing.T) {
	// Tuesday to Friday: the range must grow to the Sunday before and the
	// Saturday after, so buildings are never cut in half.
	cal := NormalizeRange(sparseCalendar(nil), mustDate(t, "2026-11-17"), mustDate(t, "2026-11-20"))

	if len(cal.Weeks) != 1 {
		t.Fatalf("weeks = %d, want 1", len(cal.Weeks))
	}
	got := weekDates(t, cal, 0)
	want := []string{"2026-11-15", "2026-11-16", "2026-11-17", "2026-11-18", "2026-11-19", "2026-11-20", "2026-11-21"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("day %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestNormalizeRangeLeapDay(t *testing.T) {
	// 2024 is a leap year: Feb 29 must appear as an ordinary day.
	cal := NormalizeRange(sparseCalendar(map[string]int{"2024-02-29": 7}),
		mustDate(t, "2024-02-28"), mustDate(t, "2024-03-01"))

	got := weekDates(t, cal, 0)
	want := []string{"2024-02-25", "2024-02-26", "2024-02-27", "2024-02-28", "2024-02-29", "2024-03-01", "2024-03-02"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("day %d = %s, want %s", i, got[i], want[i])
		}
	}
	if cal.Total != 7 {
		t.Errorf("total = %d, want 7", cal.Total)
	}
	for _, d := range cal.Weeks[0] {
		if d.Date.Format("2006-01-02") == "2024-02-29" && d.Count != 7 {
			t.Errorf("leap day count = %d, want 7", d.Count)
		}
	}
}

func TestNormalizeRangeCrossYearWeekStaysOneBuilding(t *testing.T) {
	// The week containing both Dec 31 and Jan 1 is a single week, so the
	// year boundary must not split it into two buildings.
	cal := NormalizeRange(sparseCalendar(nil), mustDate(t, "2024-12-31"), mustDate(t, "2025-01-02"))

	if len(cal.Weeks) != 1 {
		t.Fatalf("weeks = %d, want 1 (the year boundary must not split the week)", len(cal.Weeks))
	}
	got := weekDates(t, cal, 0)
	if got[0] != "2024-12-29" || got[6] != "2025-01-04" {
		t.Fatalf("week runs %s..%s, want 2024-12-29..2025-01-04", got[0], got[6])
	}
}

func TestNormalizeRangeFillsMissingDaysWithZero(t *testing.T) {
	cal := NormalizeRange(sparseCalendar(map[string]int{"2026-11-16": 3}),
		mustDate(t, "2026-11-16"), mustDate(t, "2026-11-18"))

	byDate := map[string]github.Day{}
	for _, d := range cal.Weeks[0] {
		byDate[d.Date.Format("2006-01-02")] = d
	}
	if d := byDate["2026-11-16"]; d.Count != 3 {
		t.Errorf("known day count = %d, want 3", d.Count)
	}
	for _, date := range []string{"2026-11-15", "2026-11-17", "2026-11-18"} {
		if d := byDate[date]; d.Count != 0 || d.Level != 0 {
			t.Errorf("missing day %s = %+v, want zero count and level", date, d)
		}
	}
	if cal.Total != 3 {
		t.Errorf("total = %d, want 3", cal.Total)
	}
}

func TestBuildCompareSharesOneScale(t *testing.T) {
	// The busiest week of the pair sets the scale for both cities.
	a := calendar("2026-01-04", [][]int{{100, 0, 0, 0, 0, 0, 0}})
	b := calendar("2026-01-04", [][]int{{50, 0, 0, 0, 0, 0, 0}})
	sa, sb := BuildCompare(a, b, Options{Height: 10})

	if got := buildingHeight(sa, 0); got != 10 {
		t.Errorf("record week height = %d, want 10", got)
	}
	if got := buildingHeight(sb, 0); got >= 10 {
		t.Errorf("half-as-busy week height = %d, want below the shared max of 10", got)
	}
}

func TestBuildCompareAlignsEqualWeeks(t *testing.T) {
	// The same weekly total must produce the same building height in either
	// city, on the same street baseline.
	a := calendar("2026-01-04", [][]int{{0, 0, 0, 0, 0, 0, 0}, {40, 0, 0, 0, 0, 0, 0}})
	b := calendar("2026-01-04", [][]int{{40, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0}})
	sa, sb := BuildCompare(a, b, Options{Height: 8})

	if len(sa.Rows) != len(sb.Rows) {
		t.Fatalf("row counts differ: %d vs %d (streets would not line up)", len(sa.Rows), len(sb.Rows))
	}
	if ha, hb := buildingHeight(sa, 1), buildingHeight(sb, 0); ha != hb {
		t.Errorf("equal totals built unequal heights: %d vs %d", ha, hb)
	}
}

func TestBuildCompareOneAntennaForThePair(t *testing.T) {
	a := calendar("2026-01-04", [][]int{{5, 0, 0, 0, 0, 0, 0}})
	b := calendar("2026-01-04", [][]int{{9, 0, 0, 0, 0, 0, 0}})
	sa, sb := BuildCompare(a, b, Options{Height: 8})

	count := func(s *Scene) int {
		n := 0
		for _, row := range s.Rows {
			for _, c := range row {
				if c.Kind == Antenna {
					n++
				}
			}
		}
		return n
	}
	if count(sa) != 0 || count(sb) != 1 {
		t.Errorf("antennas = %d and %d, want 0 and 1 (only the pair's record week)", count(sa), count(sb))
	}
}

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
