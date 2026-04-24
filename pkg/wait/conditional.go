package wait

import (
	"context"
	prand "math/rand"
	"time"
)

// Until repeatedly calls condition until it returns true, an error or until the context is cancelled.
// It retuns any error returned from condition or the cancelled context.
// delay specifies the length of time to wait before calling condition for the first time.
// interval specifies the length of time to wait between subsequent calls to condition.
// j adds jitter to delay and interval. See the documentation for JitterDuration for how j is interpreted.
func Until(ctx context.Context, condition func(context.Context) (bool, error), delay time.Duration, interval time.Duration, j float64) error {
	// Initial delay
	if delay > 0 {
		if err := WithJitter(ctx, delay, j); err != nil {
			return err
		}
	}

	// Loop, checking condition and then waiting
	for {
		done, err := condition(ctx)
		if err != nil {
			return err
		}
		if done {
			return nil
		}

		if err := WithJitter(ctx, interval, j); err != nil {
			return err
		}
	}
}

// Forever repeatedly calls fn until it returns an error or until the context is cancelled.
// It returns any error returned from fn or the cancelled context.
// delay specifies the length of time to wait before calling fn for the first time.
// interval specifies the length of time to wait between subsequent calls to fn.
// j adds jitter to delay and interval. See the documentation for JitterDuration for how j is interpreted.
func Forever(ctx context.Context, fn func(context.Context) error, delay time.Duration, interval time.Duration, j float64) error {
	return Until(ctx, func(c context.Context) (bool, error) {
		return false, fn(c)
	}, delay, interval, j)
}

// WithJitter waits for the specified interval, plus or minus a random fraction of the interval
// specified by jitter. If jitter is outside the range [0,1) it is ignored.
// The function returns after the adjusted interval or if the context is cancelled, in which case
// it returns the cancellation error.
func WithJitter(ctx context.Context, interval time.Duration, jitter float64) error {
	if interval <= 0 {
		return nil
	}

	interval = JitterDuration(interval, jitter)

	t := time.NewTimer(interval)
	defer t.Stop()

	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// JitterDuration adds some random jitter to a duration.
// It returns a random duration between d and d * (1+j).
// If j is outside the range [0,1) it is ignored.
func JitterDuration(d time.Duration, j float64) time.Duration {
	if j < 0 || j >= 1.0 {
		return d
	}
	return d + time.Duration(float64(d)*prand.Float64()*j)
}
