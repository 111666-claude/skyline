package city

import (
	"time"

	"github.com/pinkpixel-dev/skyline/internal/github"
)

// NormalizeRange reshapes a fetched calendar into full Sunday-first weeks
// covering [from, to]. The rules are the same for every source:
//
//   - the first week starts on the Sunday on or before from, the last week
//     ends on the Saturday on or after to, so a range never begins or ends
//     mid-building;
//   - days the source didn't return (Gitea gaps, partial GitHub weeks) are
//     filled with a zero-count, level-0 day;
//   - leap days and weeks that span a year boundary are ordinary days and
//     weeks — Feb 29 is just another window, and the week containing both
//     Dec 31 and Jan 1 stays a single building;
//   - Total is recomputed from the in-range days.
func NormalizeRange(cal *github.Calendar, from, to time.Time) *github.Calendar {
	from = truncateDay(from)
	to = truncateDay(to)
	start := from.AddDate(0, 0, -int(from.Weekday()))
	end := to.AddDate(0, 0, int(time.Saturday-to.Weekday()))

	byDate := map[time.Time]github.Day{}
	for _, w := range cal.Weeks {
		for _, d := range w {
			byDate[truncateDay(d.Date)] = d
		}
	}

	out := &github.Calendar{Login: cal.Login}
	var week []github.Day
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		day, ok := byDate[d]
		if !ok {
			day = github.Day{Date: d}
		}
		out.Total += day.Count
		week = append(week, day)
		if d.Weekday() == time.Saturday {
			out.Weeks = append(out.Weeks, week)
			week = nil
		}
	}
	return out
}

// BuildCompare lays out two calendars as a pair of scenes that share one
// height scale: the tallest building across both ranges is opt.Height rows,
// so the same weekly total yields the same building height in either city.
// Only the city holding the overall record week gets an antenna.
func BuildCompare(a, b *github.Calendar, opt Options) (*Scene, *Scene) {
	weeksA := selectWeeks(a.Weeks, opt.Weeks)
	weeksB := selectWeeks(b.Weeks, opt.Weeks)
	shared := max(maxTotal(weeksA), maxTotal(weeksB))
	return build(a, weeksA, opt, shared), build(b, weeksB, opt, shared)
}

func truncateDay(t time.Time) time.Time {
	return t.UTC().Truncate(24 * time.Hour)
}
