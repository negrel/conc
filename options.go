package conc

import (
	"context"
	"time"
)

// BlockOption define an option function applied to a nursery block.
type BlockOption func(cfg *nursery)

// WithContext returns a nursery block option that replace nursery context with
// the given one.
func WithContext(ctx context.Context) BlockOption {
	return func(n *nursery) {
		n.Context, n.cancel = context.WithCancel(ctx)
	}
}

// WithTimeout returns a nursery block option that wraps nursery context with
// a new one that timeout after the given duration.
func WithTimeout(timeout time.Duration) BlockOption {
	return func(n *nursery) {
		n.Context, n.cancel = context.WithTimeout(n.Context, timeout)
	}
}

// WithDeadline returns a nursery block option that wraps nursery context with
// a new one that will be canceled at `d`.
func WithDeadline(d time.Time) BlockOption {
	return func(n *nursery) {
		n.Context, n.cancel = context.WithDeadline(n.Context, d)
	}
}

// WithErrorHandler returns a nursery block option that adds an error handler to
// the block.
func WithErrorHandler(handler func(error)) BlockOption {
	return func(n *nursery) {
		n.onError = handler
	}
}

// WithCancelOnError returns a nursery block option that sets error handler to
// inner context cancel function.
func WithCancelOnError() BlockOption {
	return func(n *nursery) {
		n.onError = func(err error) {
			n.cancel()
		}
	}
}

// WithMaxGoroutines returns a nursery block option that limits the maximum
// number of goroutine running concurrently. If max is zero, number of goroutine
// is unlimited. This function panics if max is negative.
func WithMaxGoroutines(max int) BlockOption {
	return func(n *nursery) {
		if max < 0 {
			panic("max goroutine option must be a non negative integer")
		}

		// +1 because block function is a routine also.
		n.limiter = make(chan struct{}, max+1)
	}
}
