package sgo

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync/atomic"
	"time"
)

var (
	ErrNurseryDone = errors.New("nursery is done")
)

// Nursery primitive for structured concurrency. Functions that spawn goroutines
// should takes a Nursery parameter to avoid leaking go routines.
// See https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/#nurseries-a-structured-replacement-for-go-statements
type Nursery interface {
	context.Context
	Go(func())
}

type nursery struct {
	context.Context
	cancel func()
	panics chan any

	routineCount atomic.Int64
	routineDone  chan GoroutinePanic
}

func newNursery(ctx context.Context) *nursery {
	ctx, cancel := context.WithCancel(ctx)
	n := &nursery{
		Context:      ctx,
		cancel:       cancel,
		panics:       make(chan any),
		routineCount: atomic.Int64{},
		routineDone:  make(chan GoroutinePanic),
	}

	// Drain goroutines.
	go func() {
		for {
			panicValue := <-n.routineDone
			count := n.routineCount.Add(-1)
			if panicValue.Stack != "" {
				// Cancel all routines.
				n.cancel()
				n.panics <- panicValue
			}
			if count == 0 {
				close(n.routineDone)
				close(n.panics)
				n.cancel()
				break
			}
		}
	}()

	return n
}

// mustNotBeDone panics if nursery is done.
func (n *nursery) mustNotBeDone() {
	select {
	case <-n.Done():
		panic(ErrNurseryDone)
	default:
	}
}

// Go implements Nursery.
func (n *nursery) Go(routine func()) {
	n.mustNotBeDone()

	n.routineCount.Add(1)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				n.routineDone <- GoroutinePanic{
					Value: err,
					Stack: string(debug.Stack()),
				}

			}
		}()
		routine()
		n.routineDone <- GoroutinePanic{}
	}()
}

// BlockContext starts a nursery block that returns when all goroutines have
// returned. If a goroutine panic, context is canceled and panic is immediately
// forwarded without waiting for other goroutines to handle context cancellation.
func BlockContext[T any](ctx context.Context, block func(n Nursery) T) T {
	n := newNursery(ctx)
	var result T
	n.Go(func() {
		result = block(n)
	})

	// Wait for all routine to be done.
	panicValue := <-n.panics
	if panicValue != nil {
		panic(panicValue)
	}

	return result
}

// Block is an alias for BlockContext(context.Background(), ...). See BlockContext
// for more information.
func Block[T any](block func(n Nursery) T) T {
	return BlockContext(context.Background(), block)
}

// BlockTimeout is an alias for BlockContext(ctx, ...) with a timeout sets on `ctx`.
// See BlockContext for more information.
func BlockTimeout[T any](timeout time.Duration, block func(n Nursery) T) T {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return BlockContext(ctx, block)
}

// AllContext executes jobs concurrently and returns their result in
// when they all are done. Provided context is forwarded to underlying nursery.
func AllContext[T any](ctx context.Context, jobs ...func(context.Context) T) []T {
	results := make([]T, len(jobs))

	BlockContext(ctx, func(n Nursery) struct{} {
		for i, j := range jobs {
			job := j
			r := &results[i]
			n.Go(func() error {
				*r = job(ctx)
				return nil
			})
		}

		return struct{}{}
	})

	return results
}

// All executes jobs concurrently and returns their result in
// when they all are done.
func All[T any](jobs ...func(context.Context) T) []T {
	return AllContext(context.Background(), jobs...)
}

// Result is a struct type designed to replace (T, error) return type. It
// enable returns of error by generic function such as All().
type Result[T any] struct {
	Ok  T
	Err error
}

// GoroutinePanic holds value from a recovered panic along a stacktrace.
type GoroutinePanic struct {
	Value any
	Stack string
}

// String implements fmt.Stringer.
func (gp GoroutinePanic) String() string {
	return fmt.Sprintf("%v\n%v", gp.Value, gp.Stack)
}

// Error implements error.
func (gp GoroutinePanic) Error() string {
	return gp.String()
}

// Unwrap unwrap underlying error.
func (gp GoroutinePanic) Unwrap() error {
	if err, isErr := gp.Value.(error); isErr {
		return err
	}

	return nil
}
