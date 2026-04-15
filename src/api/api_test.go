package api

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseTimeParamsRejectsInvertedRange(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/requests/summary?start=2026-02-10&end=2026-02-01", nil)

	_, _, err := parseTimeParams(req)
	if err == nil {
		t.Fatal("parseTimeParams should reject a start date after the end date")
	}
	if got := err.Error(); got != "start date must be on or before end date" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestParseTimeParamsDefaultsToTodayUTC(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/requests/summary", nil)

	start, end, err := parseTimeParams(req)
	if err != nil {
		t.Fatalf("parseTimeParams returned error: %v", err)
	}

	now := time.Now().UTC()
	expectedStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	expectedEnd := expectedStart.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	if !start.Equal(expectedStart) {
		t.Fatalf("start = %s, want %s", start, expectedStart)
	}
	if !end.Equal(expectedEnd) {
		t.Fatalf("end = %s, want %s", end, expectedEnd)
	}
}

func TestAverageResolvedDowntimeHours(t *testing.T) {
	if got := averageResolvedDowntimeHours(180, 2); got != 1.5 {
		t.Fatalf("averageResolvedDowntimeHours(180, 2) = %v, want 1.5", got)
	}
	if got := averageResolvedDowntimeHours(60, 0); got != 0 {
		t.Fatalf("averageResolvedDowntimeHours(60, 0) = %v, want 0", got)
	}
}
