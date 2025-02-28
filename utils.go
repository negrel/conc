package conc

import (
	"context"
	"iter"
	"sync"
	"sync/atomic"
	"time"
)

// Sleep is an alternative to time.Sleep that returns once d time is elapsed or
// context is done.
func Sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// IsDone returns whether provided context is done.
func IsDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

type Job[T any] func(context.Context) (T, error)

// All executes all jobs in separate goroutines and stores each result in
// the returned slice.
func All[T any](jobs []Job[T], opts ...BlockOption) ([]T, error) {
	results := make([]T, len(jobs), len(jobs)) //nolint:gosimple

	err := Block(func(n Nursery) error {
		for i, job := range jobs {
			r := &results[i]
			n.Go(func() (err error) {
				*r, err = job(n)
				return err
			})
		}

		return nil
	}, opts...)

	return results, err
}

// Race executes all jobs in separate goroutines and returns first result.
// Remaining goroutines are canceled.
func Race[T any](jobs []Job[T], opts ...BlockOption) (T, error) {
	var first atomic.Bool
	var result T

	err := Block(func(n Nursery) error {
		for _, job := range jobs {
			n.Go(func() (err error) {
				r, err := job(n)
				if first.CompareAndSwap(false, true) {
					result = r
					n.(*nursery).cancel()
				}
				return err
			})
		}

		return nil
	}, opts...)

	return result, err
}

// Range iterates over a sequence and pass each value to a separate goroutine.
func Range[T any](seq iter.Seq[T], block func(context.Context, T) error, opts ...BlockOption) error {
	return Block(func(n Nursery) error {
		for v := range seq {
			value := v
			n.Go(func() error {
				return block(n, value)
			})
		}

		return nil
	}, opts...)
}

// Range2 is the same as Range except it uses a iter.Seq2 instead of iter.Seq.
func Range2[K, V any](seq iter.Seq2[K, V], block func(context.Context, K, V) error, opts ...BlockOption) error {
	return Block(func(n Nursery) error {
		for k, v := range seq {
			key := k
			value := v
			n.Go(func() error {
				return block(n, key, value)
			})
		}

		return nil
	}, opts...)
}

// Map applies f to each element of input and returns a new slice containing
// mapped results.
func Map[T any, V any](input []T, f func(context.Context, T) (V, error), opts ...BlockOption) ([]V, error) {
	results := make([]V, len(input), len(input)) //nolint:gosimple
	err := doMap(input, results, f, opts...)
	return results, err
}

// MapInPlace applies f to each element of input and returns modified input slice.
func MapInPlace[T any](input []T, f func(context.Context, T) (T, error), opts ...BlockOption) ([]T, error) {
	err := doMap(input, input, f, opts...)
	return input, err
}

func doMap[T any, V any](input []T, results []V, f func(context.Context, T) (V, error), opts ...BlockOption) error {
	return Block(func(n Nursery) error {
		for i, v := range input {
			value := v
			r := &results[i]
			n.Go(func() (err error) {
				*r, err = f(n, value)
				return err
			})
		}

		return nil
	}, opts...)
}

// Map2 applies f to each key, value pair of input and returns a new slice containing
// mapped results.
func Map2[K comparable, V any](input map[K]V, f func(context.Context, K, V) (K, V, error), opts ...BlockOption) (map[K]V, error) {
	var mu sync.Mutex

	results := make(map[K]V)
	return results, Block(func(n Nursery) error {
		for k, v := range input {
			key := k
			value := v
			n.Go(func() error {
				newK, newV, err := f(n, key, value)
				mu.Lock()
				results[newK] = newV
				mu.Unlock()
				return err
			})
		}

		return nil
	}, opts...)
}

// AsyncMap concurrently transforms an input sequence into an output sequence.
// The order of the output sequence is non-deterministic. Items are delivered as they finish.
//
// Within the parent nursery, a goroutine is launched which creates a nested nursery.
// The sequence is processed entirely within this nested nursery. By default, the parent
// nursery is used as the context for the nested nursery.
//
// If a transform function returns an error, that error is propagated to the parent nursery
// and the nested nursery's context is canceled, stopping processing of all remaining items.
//
// By default, AsyncMap uses an unbuffered channel for outputs. For high-throughput scenarios
// where producers might temporarily outpace consumers, use WithOutputBuffer to specify
// a buffer size.
//
// If consuming from the output sequence stops before it's fully drained (e.g.,
// breaking out of the range loop early), transformation goroutines may block indefinitely.
// To prevent this, cancel the parent nursery's context when done consuming. For multi-stage
// pipelines requiring finer control, provide a separate cancellable context via WithContext.
//
// Example:
//
//	_ = Block(func(n Nursery) error {
//		inputs := slices.Values([]int{1, 2, 3, 4, 5})
//		outputs := conc.AsyncMap(n, inputs, func(_ context.Context, i int) (string, error) {
//			return fmt.Sprintf("item-%d", i), nil
//		})
//
//		for output := range outputs {
//			fmt.Println(output)
//		}
//	})
//
// Note: If supplying a WithContext option, it will override the default parent context.
// Ensure any custom context derives from the parent nursery context to maintain proper
// cancellation propagation.
func AsyncMap[I any, O any](
	parent Nursery,
	inputs iter.Seq[I],
	transform func(context.Context, I) (O, error),
	opts ...BlockOption,
) iter.Seq[O] {
	// Create a temporary nursery to get correct output buffer size
	tn := newNursery()
	for _, opt := range opts {
		opt(tn)
	}
	var outputs chan O
	if tn.bufSize > 0 {
		outputs = make(chan O, tn.bufSize)
	} else {
		outputs = make(chan O)
	}
	allOpts := append([]BlockOption{WithContext(parent)}, opts...)

	// Core processing loop
	process := func(nested Nursery) error {
		done := nested.Done()
		for input := range inputs {
			select {
			case <-done:
				return nil
			default:
				input := input
				nested.Go(func() error {
					output, err := transform(nested, input)
					if err != nil {
						return err
					}
					select {
					case outputs <- output:
						return nil
					case <-done:
						return nested.Err()
					}
				})
			}
		}
		return nil
	}

	// Launch
	parent.Go(func() error {
		defer func() {
			close(outputs)
		}()
		if err := Block(process, allOpts...); err != nil {
			return err
		}
		return nil
	})

	return chanSeq(outputs)
}

// chanSeq returns an iterator that yields the values of ch until it is closed.
func chanSeq[T any](ch <-chan T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range ch {
			if !yield(v) {
				return
			}
		}
	}
}
