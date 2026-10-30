package store

import (
	"context"
	"time"
)

// RetryAfter reports how long the caller must wait before another attempt,
// or 0 if none of the limits has been reached. Time is taken from the database
// clock so every instance agrees.
func (p *Postgres) RetryAfter(ctx context.Context, window time.Duration, limits []Limit) (time.Duration, error) {
	keys := make([]string, len(limits))
	maxes := make([]int32, len(limits))
	for i, l := range limits {
		keys[i], maxes[i] = l.Key, int32(l.Max)
	}
	var secs float64
	err := p.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(EXTRACT(EPOCH FROM (f.window_start + make_interval(secs => $2) - now()))), 0)::float8
		FROM login_failures f
		JOIN unnest($1::text[], $3::int[]) AS l(key, max) ON l.key = f.key
		WHERE f.failures >= l.max AND f.window_start + make_interval(secs => $2) > now()`,
		keys, window.Seconds(), maxes).Scan(&secs)
	return time.Duration(secs * float64(time.Second)), err
}

// RecordFailure counts one failure against each key, starting a fresh window
// for keys whose previous window has ended. Rows untouched for a day are
// deleted in the same statement, so the table does not grow without bound.
func (p *Postgres) RecordFailure(ctx context.Context, window time.Duration, keys ...string) error {
	_, err := p.pool.Exec(ctx, `
		WITH purge AS (DELETE FROM login_failures WHERE window_start < now() - interval '1 day')
		INSERT INTO login_failures AS f (key, failures, window_start)
		SELECT k, 1, now() FROM unnest($1::text[]) AS k
		ON CONFLICT (key) DO UPDATE SET
			failures     = CASE WHEN f.window_start + make_interval(secs => $2) <= now() THEN 1 ELSE f.failures + 1 END,
			window_start = CASE WHEN f.window_start + make_interval(secs => $2) <= now() THEN now() ELSE f.window_start END`,
		keys, window.Seconds())
	return err
}

func (p *Postgres) Clear(ctx context.Context, key string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM login_failures WHERE key = $1`, key)
	return err
}
