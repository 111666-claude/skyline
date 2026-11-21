package main

import (
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

func TestParseCompareValid(t *testing.T) {
	ranges, err := parseCompare("2024-01-01..2024-12-31,2025-01-01..2025-12-31", testNow)
	if err != nil {
		t.Fatal(err)
	}
	if got := ranges[0].from.Format("2006-01-02"); got != "2024-01-01" {
		t.Errorf("first start = %s", got)
	}
	if got := ranges[1].to.Format("2006-01-02"); got != "2025-12-31" {
		t.Errorf("second end = %s", got)
	}
}

func TestParseCompareEmptyRange(t *testing.T) {
	_, err := parseCompare("2025-12-31..2025-01-01,2026-01-01..2026-06-30", testNow)
	if err == nil || !strings.Contains(err.Error(), "empty range") {
		t.Fatalf("reversed range error = %v", err)
	}
}

func TestParseCompareOverlap(t *testing.T) {
	for _, arg := range []string{
		"2024-06-01..2025-06-30,2025-01-01..2025-12-31", // partial overlap
		"2025-06-01..2025-12-31,2025-01-01..2025-07-31", // overlap in either order
		"2025-01-01..2025-06-30,2025-06-30..2025-12-31", // sharing one day overlaps
	} {
		if _, err := parseCompare(arg, testNow); err == nil || !strings.Contains(err.Error(), "overlap") {
			t.Errorf("parseCompare(%q) error = %v, want an overlap error", arg, err)
		}
	}
	// Touching but not overlapping is fine.
	if _, err := parseCompare("2024-01-01..2024-12-31,2025-01-01..2025-12-31", testNow); err != nil {
		t.Errorf("adjacent ranges should pass, got %v", err)
	}
}

func TestParseCompareFuture(t *testing.T) {
	// An end date past today is clamped to today.
	ranges, err := parseCompare("2026-01-01..2026-06-30,2026-11-01..2099-12-31", testNow)
	if err != nil {
		t.Fatal(err)
	}
	if got := ranges[1].to.Format("2006-01-02"); got != "2026-11-22" {
		t.Errorf("future end clamped to %s, want today (2026-11-22)", got)
	}

	// A range entirely in the future is empty and an error.
	_, err = parseCompare("2024-01-01..2024-12-31,2027-01-01..2027-12-31", testNow)
	if err == nil || !strings.Contains(err.Error(), "empty range") {
		t.Fatalf("future range error = %v", err)
	}
}

func TestParseCompareShape(t *testing.T) {
	for _, arg := range []string{
		"2024-01-01..2024-12-31", // one range
		"2024-01-01..2024-12-31,2025-01-01..2025-12-31,2026-01-01..2026-12-31", // three
		"2024-01-01,2025-01-01..2025-12-31",                                    // missing ..
		"not-a-date..2024-12-31,2025-01-01..2025-12-31",                        // bad date
	} {
		if _, err := parseCompare(arg, testNow); err == nil {
			t.Errorf("parseCompare(%q) should fail", arg)
		}
	}
}

func TestCompareTitles(t *testing.T) {
	ranges, err := parseCompare("2024-01-01..2024-12-31,2025-01-01..2025-12-31", testNow)
	if err != nil {
		t.Fatal(err)
	}

	a, b, err := compareTitles("", ranges)
	if err != nil {
		t.Fatal(err)
	}
	if a != "2024-01-01 → 2024-12-31" || b != "2025-01-01 → 2025-12-31" {
		t.Errorf("default titles = %q, %q", a, b)
	}

	a, b, err = compareTitles("last year, this year", ranges)
	if err != nil {
		t.Fatal(err)
	}
	if a != "last year" || b != "this year" {
		t.Errorf("custom titles = %q, %q", a, b)
	}

	for _, bad := range []string{"only one", ",", "one,"} {
		if _, _, err := compareTitles(bad, ranges); err == nil {
			t.Errorf("compareTitles(%q) should fail", bad)
		}
	}
}
