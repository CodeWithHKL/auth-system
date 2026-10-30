package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// Runs against a real Postgres, since the limiter is mostly SQL:
//
//	TEST_DATABASE_URL=postgres://myapp:myapp@localhost:5433/myapp?sslmode=disable go test ./internal/store
func TestLoginLimiter(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run")
	}
	ctx := context.Background()
	db, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close) // registered first, so it runs after the key cleanup below
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	id := time.Now().UnixNano()
	a, b, c := fmt.Sprintf("t%d:a", id), fmt.Sprintf("t%d:b", id), fmt.Sprintf("t%d:c", id)
	t.Cleanup(func() { db.Clear(ctx, a); db.Clear(ctx, b); db.Clear(ctx, c) })

	wait := func(window time.Duration, limits ...Limit) time.Duration {
		t.Helper()
		d, err := db.RetryAfter(ctx, window, limits)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}

	if d := wait(time.Hour, Limit{a, 1}); d != 0 {
		t.Fatalf("unknown key blocked for %v", d)
	}

	// Two keys are counted together; blocking starts exactly at Max.
	for i := 0; i < 3; i++ {
		if err := db.RecordFailure(ctx, time.Hour, a, b); err != nil {
			t.Fatal(err)
		}
	}
	if d := wait(time.Hour, Limit{a, 4}, Limit{b, 4}); d != 0 {
		t.Errorf("3 failures, max 4: blocked for %v", d)
	}
	d := wait(time.Hour, Limit{a, 3})
	if d <= 59*time.Minute || d > time.Hour {
		t.Errorf("3 failures, max 3: retry after %v, want just under 1h", d)
	}
	// Any one limit being hit is enough, and the longest wait is reported.
	if d := wait(time.Hour, Limit{c, 1}, Limit{b, 3}); d == 0 {
		t.Error("expected block from the second limit")
	}

	if err := db.Clear(ctx, a); err != nil {
		t.Fatal(err)
	}
	if d := wait(time.Hour, Limit{a, 1}); d != 0 {
		t.Errorf("after Clear: blocked for %v", d)
	}
	if d := wait(time.Hour, Limit{b, 3}); d == 0 {
		t.Error("Clear(a) must not clear b")
	}

	// An expired window stops blocking, and the next failure starts a new count at 1.
	window := time.Second
	db.RecordFailure(ctx, window, c)
	db.RecordFailure(ctx, window, c)
	if d := wait(window, Limit{c, 2}); d == 0 {
		t.Fatal("expected block inside the window")
	}
	time.Sleep(1200 * time.Millisecond)
	if d := wait(window, Limit{c, 2}); d != 0 {
		t.Errorf("window over but blocked for %v", d)
	}
	db.RecordFailure(ctx, window, c)
	if d := wait(window, Limit{c, 2}); d != 0 {
		t.Errorf("count did not reset after the window: blocked for %v", d)
	}
}
