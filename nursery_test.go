package sgo

import (
	"context"
	"errors"
	"fmt"
	"io"
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
			n.Go(func() error {
				time.Sleep(10 * time.Millisecond)
				return nil
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
				n.Go(func() error {
					panic("foo")
					return nil
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
			n.Go(func() error {
				time.Sleep(50 * time.Millisecond)
				return nil
			})
			n.Go(func() error {
				time.Sleep(50 * time.Millisecond)
				return nil
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
					n.Go(func() error {
						wg.Done()
						return nil
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

	t.Run("LastGoroutineCancelContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		BlockContext(ctx, func(n Nursery) error {
			n.Go(func() error {
				time.Sleep(10 * time.Millisecond)
				cancel()
				return nil
			})

			return nil
		})
	})

	t.Run("CancelBlock", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()

		start := time.Now()
		BlockContext(ctx, func(n Nursery) error {
			select {
			case <-time.After(time.Second):
			case <-n.Done():
			}

			return nil
		})

		if time.Since(start) > 10*time.Millisecond {
			t.Fatal("failed to cancel block")
		}
	})
}

func TestAll(t *testing.T) {
	t.Run("ResultsOrderIsPreserved", func(t *testing.T) {
		results := All(func(ctx context.Context) int {
			time.Sleep(time.Millisecond)
			return 1
		}, func(ctx context.Context) int {
			return 2
		})
		if results[0] != 1 || results[1] != 2 {
			t.Fatal("results order is not preserved")
		}
	})
}

func BenchmarkNursery(b *testing.B) {
	b.Run("EmptyBlock", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			Block(func(n Nursery) error {
				return nil
			})
		}
	})

	for _, routine := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("WithRoutines/%d/NoError", routine), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Block(func(n Nursery) error {
					for j := 0; j < routine; j++ {
						n.Go(func() error {
							return nil
						})
					}
					return nil
				})
			}
		})
	}

	for _, routine := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("WithRoutines/%d/Error", routine), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := Block(func(n Nursery) error {
					for j := 0; j < routine; j++ {
						n.Go(func() error {
							return io.EOF
						})
					}
					return nil
				})
				if err == nil {
					b.Fatal("block returned nil error")
				}
			}
		})
	}
}
