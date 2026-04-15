package billing

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNextMonthlyBillingRun(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "day before month boundary",
			now:  time.Date(2026, time.January, 31, 23, 59, 0, 0, time.UTC),
			want: time.Date(2026, time.February, 1, 0, 5, 0, 0, time.UTC),
		},
		{
			name: "first day before scheduled minute",
			now:  time.Date(2026, time.February, 1, 0, 3, 0, 0, time.UTC),
			want: time.Date(2026, time.February, 1, 0, 5, 0, 0, time.UTC),
		},
		{
			name: "exact scheduled minute advances to next month",
			now:  time.Date(2026, time.February, 1, 0, 5, 0, 0, time.UTC),
			want: time.Date(2026, time.March, 1, 0, 5, 0, 0, time.UTC),
		},
		{
			name: "after scheduled minute advances to next month",
			now:  time.Date(2026, time.February, 1, 0, 10, 0, 0, time.UTC),
			want: time.Date(2026, time.March, 1, 0, 5, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		if got := nextMonthlyBillingRun(test.now); !got.Equal(test.want) {
			t.Fatalf("%s: nextMonthlyBillingRun(%s) = %s, want %s", test.name, test.now, got, test.want)
		}
	}
}

func TestMonthCompleteUsesSanitizedFilenames(t *testing.T) {
	month := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)
	monthDir := t.TempDir()
	summary := &Summary{
		Members: map[string]MemberCost{
			"Alice/Bob": {MemberName: "Alice/Bob"},
		},
	}

	overviewName := filepath.Join(monthDir, "2026_02-Monthly_Overview.pdf")
	memberName := filepath.Join(monthDir, "2026_02-IBP-Service_"+sanitizeFilename("Alice/Bob")+".pdf")
	if err := os.WriteFile(overviewName, []byte("overview"), 0644); err != nil {
		t.Fatalf("write overview: %v", err)
	}
	if err := os.WriteFile(memberName, []byte("member"), 0644); err != nil {
		t.Fatalf("write member pdf: %v", err)
	}

	if !monthComplete(monthDir, month, summary) {
		t.Fatalf("monthComplete should accept the same sanitized member filename used by PDF generation")
	}
}
