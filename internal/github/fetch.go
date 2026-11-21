// Package github fetches a user's contribution calendar from the GraphQL API.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// endpoint is a var so tests can point it at a local server.
var endpoint = "https://api.github.com/graphql"

const query = `query($login: String!) {
  user(login: $login) {
    contributionsCollection {
      contributionCalendar {
        totalContributions
        weeks { contributionDays { contributionCount contributionLevel date } }
      }
    }
  }
}`

const rangeQuery = `query($login: String!, $from: DateTime!, $to: DateTime!) {
  user(login: $login) {
    contributionsCollection(from: $from, to: $to) {
      contributionCalendar {
        totalContributions
        weeks { contributionDays { contributionCount contributionLevel date } }
      }
    }
  }
}`

// Day is one square of the contribution graph.
type Day struct {
	Date  time.Time
	Count int
	Level int // 0-4, matching GitHub's four quartiles plus "none"
}

// Calendar is a year of contributions grouped by week (Sunday first).
type Calendar struct {
	Login string
	Total int
	Weeks [][]Day
}

var levels = map[string]int{
	"NONE":            0,
	"FIRST_QUARTILE":  1,
	"SECOND_QUARTILE": 2,
	"THIRD_QUARTILE":  3,
	"FOURTH_QUARTILE": 4,
}

// RateLimitError means the API refused the call because the quota is spent.
type RateLimitError struct {
	Reset time.Time // when the quota refills; zero if the server didn't say
}

func (e *RateLimitError) Error() string {
	if e.Reset.IsZero() {
		return "github api: rate limited; try again later"
	}
	return fmt.Sprintf("github api: rate limited; quota resets at %s",
		e.Reset.Local().Format("15:04"))
}

// Token returns GITHUB_TOKEN / GH_TOKEN, falling back to the gh CLI login.
func Token() (string, error) {
	for _, key := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if t := strings.TrimSpace(os.Getenv(key)); t != "" {
			return t, nil
		}
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err == nil {
		if t := strings.TrimSpace(string(out)); t != "" {
			return t, nil
		}
	}
	return "", errors.New("no GitHub token: set GITHUB_TOKEN or run `gh auth login`")
}

// Fetch downloads the last year of contributions for login.
func Fetch(ctx context.Context, token, login string) (*Calendar, error) {
	return queryCalendar(ctx, token, query, map[string]string{"login": login}, login)
}

// FetchRange downloads contributions for login between from and to
// (inclusive, UTC). The returned weeks follow GitHub's layout; use
// city.NormalizeRange to pad them to full Sunday-first weeks.
func FetchRange(ctx context.Context, token, login string, from, to time.Time) (*Calendar, error) {
	vars := map[string]string{
		"login": login,
		"from":  from.UTC().Format(time.RFC3339),
		"to":    to.UTC().Format(time.RFC3339),
	}
	return queryCalendar(ctx, token, rangeQuery, vars, login)
}

func queryCalendar(ctx context.Context, token, q string, vars map[string]string, login string) (*Calendar, error) {
	body, _ := json.Marshal(map[string]any{"query": q, "variables": vars})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusTooManyRequests {
		return nil, rateLimitError(res.Header)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api: %s", res.Status)
	}

	var payload struct {
		Data struct {
			User *struct {
				ContributionsCollection struct {
					ContributionCalendar struct {
						TotalContributions int
						Weeks              []struct {
							ContributionDays []struct {
								ContributionCount int
								ContributionLevel string
								Date              string
							}
						}
					}
				}
			}
		}
		Errors []struct {
			Type    string
			Message string
		}
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(payload.Errors) > 0 {
		if payload.Errors[0].Type == "RATE_LIMITED" {
			return nil, &RateLimitError{}
		}
		return nil, fmt.Errorf("github api: %s", payload.Errors[0].Message)
	}
	if payload.Data.User == nil {
		return nil, fmt.Errorf("user %q not found", login)
	}

	cal := payload.Data.User.ContributionsCollection.ContributionCalendar
	out := &Calendar{Login: login, Total: cal.TotalContributions}
	for _, w := range cal.Weeks {
		week := make([]Day, 0, len(w.ContributionDays))
		for _, d := range w.ContributionDays {
			date, err := time.Parse("2006-01-02", d.Date)
			if err != nil {
				return nil, fmt.Errorf("bad date %q: %w", d.Date, err)
			}
			week = append(week, Day{Date: date, Count: d.ContributionCount, Level: levels[d.ContributionLevel]})
		}
		out.Weeks = append(out.Weeks, week)
	}
	return out, nil
}

// rateLimitError builds a RateLimitError from the response headers.
func rateLimitError(h http.Header) error {
	rle := &RateLimitError{}
	if s := h.Get("X-RateLimit-Reset"); s != "" {
		if unix, err := strconv.ParseInt(s, 10, 64); err == nil {
			rle.Reset = time.Unix(unix, 0)
		}
	}
	return rle
}
