package conc

import (
	"context"
	"sync"
	"time"
)

// BlockOption define an option function applied to a nursery block.
type BlockOption func(cfg *nursery)

// WithContext returns a nursery block option that replaces nursery context with
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
		ctx := n.Context
		if ctx == nil {
			ctx = context.Background()
		}
		n.Context, n.cancel = context.WithTimeout(ctx, timeout)
	}
}

// WithDeadline returns a nursery block option that wraps nursery context with
// a new one that will be canceled at `d`.
func WithDeadline(d time.Time) BlockOption {
	return func(n *nursery) {
		ctx := n.Context
		if ctx == nil {
			ctx = context.Background()
		}
		n.Context, n.cancel = context.WithDeadline(ctx, d)
	}
}

// WithErrorHandler returns a nursery block option that adds an error handler to
// the block. Provided error handler is executed in the goroutine that returned
// the error.
func WithErrorHandler(handler func(error)) BlockOption {
	return func(n *nursery) {
		n.onError = handler
	}
}

// WithCollectErrors returns a nursery block option that sets error handler to
// collect goroutine errors into provided error slice. Provided error slice must
// not be read and write until end of block.
func WithCollectErrors(errors *[]error) BlockOption {
	mu := &sync.Mutex{}
	return func(n *nursery) {
		n.onError = func(err error) {
			mu.Lock()
			*errors = append(*errors, err)
			mu.Unlock()
		}
	}
}

// WithIgnoreErrors returns a nursery block option that sets error handler to a
// noop function.
func WithIgnoreErrors() BlockOption {
	return func(n *nursery) {
		n.onError = func(err error) {}
	}
}

// WithMaxGoroutines returns a nursery block option that limits the maximum
// number of goroutine running concurrently. If max is zero, number of goroutine
// is unlimited. This function panics if max is negative.
func WithMaxGoroutines(max int) BlockOption {
	return func(n *nursery) {
		if max < 0 {
			panic("max goroutine option must be a non negative integer")
		} else if max == 0 {
			n.limiter = nil
			return
		}

		// +1 because block function is a routine also.
		n.limiter = make(chan struct{}, max+1)
	}
}

// WithOutputBuffer returns a nursery block option that sets the output buffer
// size for AsyncMap operations. This option is only used by AsyncMap and is
// ignored by other functions. If size is zero or negative, an unbuffered
// channel is used (default behavior).
func WithOutputBuffer(size int) BlockOption {
	return func(n *nursery) {
		if size < 0 {
			size = 0
		}
		n.bufSize = size
	}
}
