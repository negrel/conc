package sgo

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNursery(t *testing.T) {
	t.Run("EmptyBlock", func(t *testing.T) {
		Block(func(n Nursery) error {
			return nil
		})
	})

	t.Run("SleepInBlock", func(t *testing.T) {
		start := time.Now()
		Block(func(n Nursery) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
		if time.Since(start) < 10*time.Millisecond {
			t.Fatal("block returned before end of sleep")
		}
	})

	t.Run("SleepInGoroutine", func(t *testing.T) {
		start := time.Now()
		Block(func(n Nursery) error {
			n.Go(func() {
				time.Sleep(10 * time.Millisecond)
			})
			return nil
		})
		if time.Since(start) < 10*time.Millisecond {
			t.Fatal("block returned before end of sleep")
		}
	})

	t.Run("PanicInBlock", func(t *testing.T) {
		var panicValue any

		func() {
			defer func() {
				if v := recover(); v != nil {
					panicValue = v
				}
			}()

			Block(func(n Nursery) error {
				panic("foo")
			})
		}()

		if panicValue.(GoroutinePanic).Value != "foo" {
			t.Fatal("panic not forwarded")
		}
	})

	t.Run("PanicInGoroutine", func(t *testing.T) {
		var panicValue any

		func() {
			defer func() {
				if v := recover(); v != nil {
					panicValue = v
				}
			}()

			Block(func(n Nursery) error {
				n.Go(func() {
					panic("foo")
				})
				return nil
			})
		}()

		if panicValue.(GoroutinePanic).Value != "foo" {
			t.Fatal("panic not forwarded")
		}
	})

	t.Run("ConcurrentWork", func(t *testing.T) {
		start := time.Now()
		Block(func(n Nursery) error {
			n.Go(func() {
				time.Sleep(50 * time.Millisecond)
			})
			n.Go(func() {
				time.Sleep(50 * time.Millisecond)
			})
			return nil
		})
		if time.Since(start) >= 100*time.Millisecond {
			t.Fatal("routines aren't executed concurrently")
		}
	})

	t.Run("GoAfterEndOfBlock", func(t *testing.T) {
		var panicValue any

		func() {
			var wg sync.WaitGroup
			wg.Add(1)

			Block(func(n Nursery) error {
				go func() {
					defer func() {
						if v := recover(); v != nil {
							panicValue = v
							defer wg.Done()
						}
					}()

					time.Sleep(time.Millisecond)
					n.Go(func() {
						wg.Done()
					})
				}()
				return nil
			})

			wg.Wait()
		}()

		if panicValue == nil {
			t.Fatal("use of nursery after end of block didn't panic")
		}
		if !errors.Is(panicValue.(error), ErrNurseryDone) {
			t.Fatal("use of nursery after end of block didn't panicked with ErrNurseryDone")
		}
	})
}
