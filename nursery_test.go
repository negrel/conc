package conc

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

func TestNursery(t *testing.T) {
	t.Run("EmptyBlock", func(t *testing.T) {
		err := Block(func(n Nursery) error {
			return nil
		})
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("SleepInBlock", func(t *testing.T) {
		start := time.Now()
		err := Block(func(n Nursery) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
		if err != nil {
			t.Error(err)
		}
		if time.Since(start) < 10*time.Millisecond {
			t.Fatal("block returned before end of sleep")
		}
	})

	t.Run("SleepInGoroutine", func(t *testing.T) {
		start := time.Now()
		err := Block(func(n Nursery) error {
			n.Go(func() error {
				time.Sleep(10 * time.Millisecond)
				return nil
			})
			return nil
		})
		if err != nil {
			t.Error(err)
		}
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

			err := Block(func(n Nursery) error {
				panic("foo")
			})
			if err != nil {
				t.Error(err)
			}
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

			err := Block(func(n Nursery) error {
				n.Go(func() error {
					panic("foo")
				})
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}()

		if panicValue.(GoroutinePanic).Value != "foo" {
			t.Fatal("panic not forwarded")
		}
	})

	t.Run("ConcurrentWork", func(t *testing.T) {
		start := time.Now()
		err := Block(func(n Nursery) error {
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
		if err != nil {
			t.Error(err)
		}
		if time.Since(start) >= 100*time.Millisecond {
			t.Fatal("routines aren't executed concurrently")
		}
	})

	t.Run("GoAfterEndOfBlock", func(t *testing.T) {
		var panicValue any

		func() {
			var wg sync.WaitGroup
			wg.Add(1)

			err := Block(func(n Nursery) error {
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
			if err != nil {
				t.Error(err)
			}

			wg.Wait()
		}()

		if panicValue == nil {
			t.Fatal("use of nursery after end of block didn't panic")
		}
		if panicValue.(error).Error() != "send on closed channel" {
			t.Fatal("use of nursery after end of block didn't panicked with ErrNurseryDone")
		}
	})

	t.Run("LastGoroutineCancelContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		err := Block(func(n Nursery) error {
			n.Go(func() error {
				time.Sleep(10 * time.Millisecond)
				cancel()

				select {
				case <-n.Done():
				default:
					panic("nursery not canceled")
				}
				return nil
			})

			return nil
		}, WithContext(ctx))
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("CancelBlock", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()

		start := time.Now()
		err := Block(func(n Nursery) error {
			select {
			case <-time.After(time.Second):
			case <-n.Done():
			}

			return nil
		}, WithContext(ctx))
		if err != nil {
			t.Error(err)
		}

		if time.Since(start) > 10*time.Millisecond {
			t.Fatal("failed to cancel block")
		}
	})

	t.Run("WithMaxGoroutines", func(t *testing.T) {
		t.Run("SingleRoutine", func(t *testing.T) {
			start := time.Now()
			err := Block(func(n Nursery) error {
				for i := 0; i < 3; i++ {
					n.Go(func() error {
						time.Sleep(time.Millisecond)
						return nil
					})
				}

				return nil
			}, WithMaxGoroutines(1))
			if err != nil {
				t.Error(err)
			}

			if time.Since(start) < 3*time.Millisecond {
				t.Fatal("max goroutine parameter is ignored")
			}
		})
	})

	t.Run("WithErrorHandler/Custom", func(t *testing.T) {
		errHandlerCallCount := 0
		errIsIoEOF := false

		err := Block(func(n Nursery) error {
			n.Go(func() error {
				return io.EOF
			})

			return nil
		}, WithErrorHandler(func(err error) {
			errHandlerCallCount++
			errIsIoEOF = err == io.EOF
		}))
		if err != nil {
			t.Error(err)
		}

		if errHandlerCallCount != 1 {
			t.Fatalf("error handler called %v time(s) instead of 1 time", errHandlerCallCount)
		}
		if !errIsIoEOF {
			t.Fatalf("error handler provided error isn't io.EOF")
		}
	})
}
