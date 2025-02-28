package conc

import (
	"context"
	"iter"
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
	results := make(map[K]V)
	err := doMap2(input, results, f, opts...)
	return results, err
}

// MapInPlace2 applies f to each key, value pair of input and returns modified map.
func Map2InPlace[K comparable, V any](input map[K]V, f func(context.Context, K, V) (K, V, error), opts ...BlockOption) (map[K]V, error) {
	err := doMap2(input, input, f, opts...)
	return input, err
}

func doMap2[K comparable, V any](input map[K]V, results map[K]V, f func(context.Context, K, V) (K, V, error), opts ...BlockOption) error {
	return Block(func(n Nursery) error {
		for k, v := range input {
			key := k
			value := v
			n.Go(func() error {
				newK, newV, err := f(n, key, value)
				results[newK] = newV
				return err
			})
		}

		return nil
	}, opts...)
}
