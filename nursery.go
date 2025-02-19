package conc

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync/atomic"
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
	cancel        func()
	onError       func(error)
	errors        chan error
	limiter       limiter
	goRoutine     chan func() error
	routinesCount atomic.Int32
}

func newNursery() *nursery {
	n := &nursery{
		Context:   nil,
		cancel:    nil,
		errors:    make(chan error),
		limiter:   nil,
		goRoutine: make(chan func() error),
	}

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

	n.routinesCount.Add(1)
	if n.limiter == nil {
		select {
		case n.goRoutine <- routine:
			// Successfully reused a goroutine.
		default:
			// No goroutine available, spawn a new one.
			n.goNew(routine)
		}
	} else {
		select {
		case n.limiter <- struct{}{}:
			// We are below our limit.
			n.goNew(routine)
		case <-n.Done():
			// Context canceled.
		case n.goRoutine <- routine:
			// Successfully reused a goroutine.
		}
	}
}

func (n *nursery) goNew(routine func() error) {
	go func() {
		defer catchPanics(n.errors)
		for r := range n.goRoutine {
			n.errors <- r()
		}
	}()

	select {
	case <-n.Done():
		// Context canceled.
	case n.goRoutine <- routine:
		// routine forwarded.
	}
}

// Block starts a nursery block that returns when all goroutines have returned.
// If a goroutine panic, context is canceled and panic is immediately forwarded
// without waiting for other goroutines to handle context cancellation. Errors
// returned by goroutines are joined and returned at the end of the block.
func Block(block func(n Nursery) error, opts ...BlockOption) (err error) {
	n := newNursery()
	for _, opt := range opts {
		opt(n)
	}

	// Default context.
	if n.Context == nil {
		n.Context, n.cancel = context.WithCancel(context.Background())
	}
	defer n.cancel()

	// Start block.
	n.Go(func() error {
		return block(n)
	})

	// Event loop.
	for {
		e := <-n.errors
		if panicValue, isPanic := e.(GoroutinePanic); isPanic {
			panic(panicValue)
		}
		err = errors.Join(err, e)
		count := n.routinesCount.Add(-1)
		if count == 0 {
			close(n.goRoutine)
			close(n.errors)
			break
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

type limiter chan struct{}
