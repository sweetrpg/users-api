package models

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDailyNewUsersBucketsByUTCDay(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	windowStart := today.AddDate(0, 0, -6)
	windowEnd := today.AddDate(0, 0, 1)

	run := fmt.Sprintf("%d", time.Now().UnixNano())
	// One user 3 days ago (noon, so squarely inside that UTC day), one today, one 40 days ago
	// (outside the window).
	seedUserCreatedAt(t, "hist-3d-"+run+"@example.com", today.AddDate(0, 0, -3).Add(12*time.Hour))
	seedUserCreatedAt(t, "hist-today-"+run+"@example.com", today.Add(6*time.Hour))
	seedUserCreatedAt(t, "hist-40d-"+run+"@example.com", today.AddDate(0, 0, -40))

	daily, err := DailyNewUsers(ctx, windowStart, windowEnd)
	if err != nil {
		t.Fatalf("DailyNewUsers: %v", err)
	}

	d3 := today.AddDate(0, 0, -3).Format("2006-01-02")
	d0 := today.Format("2006-01-02")
	if daily[d3] < 1 {
		t.Errorf("day %s new-user count = %d, want >= 1", d3, daily[d3])
	}
	if daily[d0] < 1 {
		t.Errorf("day %s new-user count = %d, want >= 1", d0, daily[d0])
	}
	// The 40-day-old user is before windowStart, so it must not appear in any in-window bucket.
	if v, ok := daily[today.AddDate(0, 0, -40).Format("2006-01-02")]; ok {
		t.Errorf("out-of-window day present in map with count %d", v)
	}
}

func TestCountUsersCreatedBefore(t *testing.T) {
	ctx := context.Background()
	cutoff := time.Now().UTC().AddDate(0, 0, -10)

	before, err := CountUsersCreatedBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("CountUsersCreatedBefore baseline: %v", err)
	}

	run := fmt.Sprintf("%d", time.Now().UnixNano())
	seedUserCreatedAt(t, "before-old-"+run+"@example.com", cutoff.AddDate(0, 0, -5)) // counts
	seedUserCreatedAt(t, "before-new-"+run+"@example.com", cutoff.AddDate(0, 0, 5))  // does not

	after, err := CountUsersCreatedBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("CountUsersCreatedBefore after: %v", err)
	}
	if got := after - before; got != 1 {
		t.Errorf("delta = %d, want 1 (only the user created before the cutoff counts)", got)
	}
}
