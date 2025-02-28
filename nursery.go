package conc

import (
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

// Routine define a function executed in its own goroutine.
type Routine = func() error

// Nursery is a supervisor that executes goroutines and manages their lifecycle.
// It embeds a context.Context to provide cancellation and deadlines to all
// spawned goroutines. When the nursery's context is canceled, all goroutines
// are signaled to stop via the context cancellation.
type Nursery interface {
	context.Context

	// Executes provided [Routine] as soon as possible in a separate goroutine.
	Go(Routine)
}

type nursery struct {
	context.Context
	cancel        func()
	onError       func(error)
	errors        chan error
	limiter       limiter
	goRoutine     chan Routine
	routinesCount atomic.Int32
	bufSize       int
}

func newNursery() *nursery {
	n := &nursery{
		Context:   nil,
		cancel:    nil,
		onError:   nil,
		errors:    make(chan error),
		limiter:   nil,
		goRoutine: make(chan func() error),
		bufSize:   0,
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

// Go implements Nursery.
func (n *nursery) Go(routine func() error) {
	new := n.routinesCount.Add(1)
	if new < 2 {
		panic("use of nursery after end of block")
	}

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
		case n.goRoutine <- routine:
			// Successfully reused a goroutine.
		}
	}
}

func (n *nursery) goNew(routine Routine) {
	go func() {
		defer catchPanics(n.errors)
		for r := range n.goRoutine {
			// TODO: add option to skip routine if context is canceled.

			err := r()
			if err != nil {
				n.onError(err)
			}
			n.errors <- err
		}
	}()

	// Execute routine.
	n.goRoutine <- routine
}

// Block starts a nursery block that returns when all goroutines have returned.
// If a goroutine returns an error, it is returned after context is canceled
// unless a custom error handler is provided. In case of a panic context is
// canceled and panic is immediately forwarded without waiting for other
// goroutines to handle context cancellation. Error returned by block closure
// always trigger a context cancellation and is returned if it occurs before a
// default goroutine error handler is called.
// Calling [Nursery].Go() after end of block always panic. Calling [Nursery].Go
// after context is canceled still runs the provided function, you're responsible
// for handling cancellation.
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

	// Default error handler.
	once := sync.Once{}
	if n.onError == nil {
		n.onError = func(e error) {
			once.Do(func() {
				n.cancel()
				err = e
			})
		}
	}

	// Start block.
	n.routinesCount.Add(1) // Bypass end of block check.
	n.Go(func() error {
		e := block(n)
		if e != nil {
			once.Do(func() {
				n.cancel()
				err = e
			})
		}
		n.routinesCount.Add(-1) // Restore end of block check.
		return nil
	})

	// Event loop.
	for {
		e := <-n.errors
		if panicValue, isPanic := e.(GoroutinePanic); isPanic {
			panic(panicValue)
		}
		count := n.routinesCount.Add(-1)
		if count == 0 {
			close(n.goRoutine)
			close(n.errors)
			break
		}
	}

	return err
}

type limiter chan struct{}
