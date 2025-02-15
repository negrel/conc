package conc

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
)

var (
	ErrNurseryDone = errors.New("nursery is done")
)

// Nursery primitive for structured concurrency. Functions that spawn goroutines
// should takes a Nursery parameter to avoid leaking go routines.
// See https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/#nurseries-a-structured-replacement-for-go-statements
type Nursery interface {
	context.Context
	Go(func() error)
}

type nursery struct {
	context.Context
	cancel func()
	panics chan any
	errors chan error

	maxRoutines int
	routineDone chan error

	goRoutine chan func() error
	onError   func(error)
}

func newNursery(ctx context.Context) *nursery {
	ctx, cancel := context.WithCancel(ctx)
	n := &nursery{
		Context:   ctx,
		cancel:    cancel,
		panics:    make(chan any),
		errors:    make(chan error),
		goRoutine: make(chan func() error),
	}

	// Event loop.
	go func() {
		done := false
		routinesCount := 0
		routineDone := make(chan error)
		for !done {
			handleRoutineDone := func(routineValue error) {
				routinesCount--
				if gpanic, isPanic := routineValue.(GoroutinePanic); isPanic {
					// Cancel all routines.
					n.cancel()
					n.panics <- gpanic
				} else if routineValue != nil {
					n.errors <- routineValue
				}
				if routinesCount == 0 {
					close(routineDone)
					close(n.panics)
					close(n.errors)
					n.cancel()
					done = true
				}
			}

			// We can spawn routine.
			if routinesCount < n.maxRoutines || n.maxRoutines <= 0 {
				select {
				case routine := <-n.goRoutine:
					routinesCount++
					go func() {
						defer catchPanics(routineDone)
						err := routine()
						if err != nil && n.onError != nil {
							n.onError(err)
						}
						routineDone <- err
					}()

				case routineValue := <-routineDone:
					handleRoutineDone(routineValue)
				}
			} else {
				// We can't spawn routine.
				routineValue := <-routineDone
				handleRoutineDone(routineValue)
			}
		}
	}()

	return n
}

func catchPanics(routineDone chan<- error) {
	if err := recover(); err != nil {
		routineDone <- GoroutinePanic{
			Value: err,
			Stack: string(debug.Stack()),
		}
	}
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
func (n *nursery) Go(routine func() error) {
	n.mustNotBeDone()

	n.goRoutine <- routine
}

// Block starts a nursery block that returns when all goroutines have
// returned. If a goroutine panic, context is canceled and panic is immediately
// forwarded without waiting for other goroutines to handle context cancellation.
// Errors returned by goroutines are joined and returned at the end of the block.
func Block(block func(n Nursery) error, opts ...BlockOption) (err error) {
	n := newNursery(context.Background())

	for _, opt := range opts {
		opt(n)
	}

	n.Go(func() error {
		block(n)
		return nil
	})

	// Wait for all routine to be done.
loop:
	for {
		select {
		case panicValue := <-n.panics:
			if panicValue != nil {
				panic(panicValue)
			}
			break loop
		case e := <-n.errors:
			err = errors.Join(err, e)
		}
	}

	return err
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
