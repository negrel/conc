package conc

import (
	"context"
	"errors"
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
		t.Run("Zero", func(t *testing.T) {
			start := time.Now()
			err := Block(func(n Nursery) error {
				for i := 0; i < 3; i++ {
					n.Go(func() error {
						time.Sleep(10 * time.Millisecond)
						return nil
					})
				}
				return nil
			}, WithMaxGoroutines(0))

			if err != nil {
				t.Error(err)
			}

			// With no limit, all goroutines should run concurrently
			if time.Since(start) >= 30*time.Millisecond {
				t.Fatal("goroutines not running concurrently with zero limit")
			}
		})

		t.Run("Negative", func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("expected panic with negative max goroutines")
				}
				if r != "max goroutine option must be a non negative integer" {
					t.Fatalf("unexpected panic message: %v", r)
				}
			}()

			_ = Block(func(n Nursery) error {
				return nil
			}, WithMaxGoroutines(-1))

			t.Fatal("should have panicked")
		})

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

	t.Run("BlockReturnsError", func(t *testing.T) {
		expectedErr := errors.New("block error")
		err := Block(func(n Nursery) error {
			return expectedErr
		})
		if err != expectedErr {
			t.Fatalf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("LimiterWithContextCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		blockCh := make(chan struct{})
		doneCh := make(chan struct{})

		err := Block(func(n Nursery) error {
			// Fill the limiter to capacity with a long-running goroutine
			n.Go(func() error {
				close(blockCh)
				<-doneCh
				return nil
			})

			<-blockCh
			cancel()

			// Try to add another goroutine - this should hit the context.Done() case
			// since the limiter is full and the context is canceled
			n.Go(func() error {
				t.Fatal("this goroutine should not run")
				return nil
			})

			close(doneCh)
			return nil
		}, WithMaxGoroutines(1), WithContext(ctx))

		if err != nil {
			t.Error(err)
		}
	})

	t.Run("WithTimeout", func(t *testing.T) {
		start := time.Now()
		err := Block(func(n Nursery) error {
			select {
			case <-time.After(time.Second):
				t.Fatal("timeout not triggered")
			case <-n.Done():
				// Expected path
			}
			return nil
		}, WithTimeout(10*time.Millisecond))

		if err != nil {
			t.Error(err)
		}

		if time.Since(start) > 100*time.Millisecond {
			t.Fatal("timeout not effective")
		}
	})

	t.Run("WithDeadline", func(t *testing.T) {
		start := time.Now()
		deadline := time.Now().Add(10 * time.Millisecond)

		err := Block(func(n Nursery) error {
			select {
			case <-time.After(time.Second):
				t.Fatal("deadline not triggered")
			case <-n.Done():
				// Expected path
			}
			return nil
		}, WithDeadline(deadline))

		if err != nil {
			t.Error(err)
		}

		if time.Since(start) > 100*time.Millisecond {
			t.Fatal("deadline not effective")
		}
	})

	t.Run("WithCollectErrors", func(t *testing.T) {
		var collectedErrors []error
		expectedErr1 := errors.New("error 1")
		expectedErr2 := errors.New("error 2")

		err := Block(func(n Nursery) error {
			n.Go(func() error {
				return expectedErr1
			})
			n.Go(func() error {
				return expectedErr2
			})
			return nil
		}, WithCollectErrors(&collectedErrors))

		if err != nil {
			t.Error(err)
		}

		if len(collectedErrors) != 2 {
			t.Fatalf("expected 2 errors, got %d", len(collectedErrors))
		}

		if collectedErrors[0] == nil || collectedErrors[1] == nil {
			t.Fatalf("not all expected errors were collected: %v", collectedErrors)
		}
	})

	t.Run("WithIgnoreErrors", func(t *testing.T) {
		err := Block(func(n Nursery) error {
			n.Go(func() error {
				return errors.New("this error should be ignored")
			})
			return nil
		}, WithIgnoreErrors())

		if err != nil {
			t.Fatalf("expected nil error with WithIgnoreErrors, got: %v", err)
		}
	})
}

func TestGoroutinePanic(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		gp := GoroutinePanic{
			Value: "test panic",
			Stack: "test stack",
		}

		str := gp.String()
		if str != "test panic\ntest stack" {
			t.Fatalf("unexpected String() result: %s", str)
		}
	})

	t.Run("Error", func(t *testing.T) {
		gp := GoroutinePanic{
			Value: "test panic",
			Stack: "test stack",
		}

		errStr := gp.Error()
		if errStr != "test panic\ntest stack" {
			t.Fatalf("unexpected Error() result: %s", errStr)
		}
	})

	t.Run("Unwrap/WithError", func(t *testing.T) {
		originalErr := errors.New("original error")
		gp := GoroutinePanic{
			Value: originalErr,
			Stack: "test stack",
		}

		unwrapped := gp.Unwrap()
		if unwrapped != originalErr {
			t.Fatalf("expected unwrapped error to be %v, got %v", originalErr, unwrapped)
		}
	})

	t.Run("Unwrap/WithNonError", func(t *testing.T) {
		gp := GoroutinePanic{
			Value: "not an error",
			Stack: "test stack",
		}

		unwrapped := gp.Unwrap()
		if unwrapped != nil {
			t.Fatalf("expected unwrapped error to be nil, got %v", unwrapped)
		}
	})
}
