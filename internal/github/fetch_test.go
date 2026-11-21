package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// withServer points the GraphQL endpoint at a test server for the duration
// of one test.
func withServer(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	old := endpoint
	endpoint = srv.URL
	t.Cleanup(func() { endpoint = old })
}

func TestFetchRangeSendsDatesAndParsesWeeks(t *testing.T) {
	var gotBody map[string]any
	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("auth header = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("bad request body: %v", err)
		}
		fmt.Fprint(w, `{"data":{"user":{"contributionsCollection":{"contributionCalendar":{
			"totalContributions":5,
			"weeks":[{"contributionDays":[
				{"contributionCount":2,"contributionLevel":"FIRST_QUARTILE","date":"2025-01-05"},
				{"contributionCount":3,"contributionLevel":"SECOND_QUARTILE","date":"2025-01-06"}
			]}]
		}}}}}`)
	})

	from := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 11, 0, 0, 0, 0, time.UTC)
	cal, err := FetchRange(context.Background(), "secret", "octo", from, to)
	if err != nil {
		t.Fatal(err)
	}

	vars, ok := gotBody["variables"].(map[string]any)
	if !ok {
		t.Fatalf("request has no variables: %v", gotBody)
	}
	if vars["from"] != "2025-01-05T00:00:00Z" || vars["to"] != "2025-01-11T00:00:00Z" {
		t.Errorf("date variables = %v..%v", vars["from"], vars["to"])
	}
	if cal.Total != 5 || len(cal.Weeks) != 1 || len(cal.Weeks[0]) != 2 {
		t.Fatalf("calendar = %+v", cal)
	}
	if d := cal.Weeks[0][1]; d.Count != 3 || d.Level != 2 {
		t.Errorf("second day = %+v, want count 3 level 2", d)
	}
}

func TestFetchRateLimited(t *testing.T) {
	reset := time.Date(2026, 11, 22, 18, 0, 0, 0, time.UTC)
	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprint(reset.Unix()))
		w.WriteHeader(http.StatusForbidden)
	})

	_, err := Fetch(context.Background(), "secret", "octo")
	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("error = %v, want a RateLimitError", err)
	}
	if !rle.Reset.Equal(reset) {
		t.Errorf("reset = %s, want %s", rle.Reset, reset)
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("message = %q, want it to say rate limited", err)
	}
}

func TestFetchGraphQLRateLimited(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"errors":[{"type":"RATE_LIMITED","message":"API rate limit exceeded"}]}`)
	})

	_, err := Fetch(context.Background(), "secret", "octo")
	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("error = %v, want a RateLimitError", err)
	}
}

func TestFetchUnknownUser(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"user":null}}`)
	})

	_, err := Fetch(context.Background(), "secret", "ghost")
	if err == nil || !strings.Contains(err.Error(), `user "ghost" not found`) {
		t.Fatalf("error = %v, want a not-found error", err)
	}
}
