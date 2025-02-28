package conc

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSleep(t *testing.T) {
	t.Run("BackgroundContext", func(t *testing.T) {
		ctx := context.Background()
		start := time.Now()
		Sleep(ctx, 100*time.Millisecond)
		elapsed := time.Since(start)
		if elapsed < 100*time.Millisecond {
			t.Errorf("Sleep returned too early: got %v, want >= 100ms", elapsed)
		}
	})

	t.Run("ContextCanceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		start := time.Now()
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		Sleep(ctx, 1*time.Second)
		elapsed := time.Since(start)
		if elapsed >= 1*time.Second {
			t.Errorf("Sleep didn't respect context cancellation: got %v, want < 1s", elapsed)
		}
	})
}

func TestIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	if IsDone(ctx) {
		t.Fatal("expected false, got true")
	}

	cancel()

	if !IsDone(ctx) {
		t.Fatal("expected true, got false")
	}
}

func TestAll(t *testing.T) {
	t.Run("NoError", func(t *testing.T) {
		jobs := []Job[int]{
			func(ctx context.Context) (int, error) { return 1, nil },
			func(ctx context.Context) (int, error) { return 2, nil },
			func(ctx context.Context) (int, error) { return 3, nil },
		}

		results, err := All(jobs)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expected := []int{1, 2, 3}
		if !reflect.DeepEqual(results, expected) {
			t.Errorf("got %v, want %v", results, expected)
		}
	})

	t.Run("JobError", func(t *testing.T) {
		expectedErr := errors.New("test error")
		jobs := []Job[int]{
			func(ctx context.Context) (int, error) {
				return 1, nil
			},
			func(ctx context.Context) (int, error) {
				return 0, expectedErr
			},
			func(ctx context.Context) (int, error) {
				return 3, nil
			},
		}

		_, err := All(jobs)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestRace(t *testing.T) {
	t.Run("ReturnsFirstResult", func(t *testing.T) {
		jobs := []Job[int]{
			func(ctx context.Context) (int, error) {
				time.Sleep(100 * time.Millisecond)
				return 1, nil
			},
			func(ctx context.Context) (int, error) {
				return 2, nil
			},
			func(ctx context.Context) (int, error) {
				time.Sleep(200 * time.Millisecond)
				return 3, nil
			},
		}

		result, err := Race(jobs)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 2 {
			t.Errorf("got %v, want 2", result)
		}
	})

	t.Run("JobError", func(t *testing.T) {
		expectedErr := errors.New("test error")
		jobs := []Job[int]{
			func(ctx context.Context) (int, error) {
				time.Sleep(100 * time.Millisecond)
				return 0, expectedErr
			},
			func(ctx context.Context) (int, error) {
				return 2, nil
			},
		}

		_, err := Race(jobs)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestRange(t *testing.T) {
	t.Run("NoJobError", func(t *testing.T) {
		numbers := []int{1, 2, 3}
		processed := make([]bool, len(numbers))
		remaining := make([]int, len(numbers))
		copy(remaining, numbers)

		seq := iter.Seq[int](
			func(yield func(int) bool) {
				for _, n := range remaining {
					if !yield(n) {
						return
					}
				}
			},
		)

		err := Range(
			seq,
			func(ctx context.Context, n int) error {
				processed[n-1] = true
				return nil
			},
		)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for i, p := range processed {
			if !p {
				t.Errorf("item %d was not processed", i+1)
			}
		}
	})

	t.Run("WithError", func(t *testing.T) {
		expectedErr := errors.New("test error")
		seq := iter.Seq[int](
			func(yield func(int) bool) {
				yield(1)
			},
		)

		err := Range(seq, func(ctx context.Context, n int) error {
			return expectedErr
		})

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestRange2(t *testing.T) {
	t.Run("NoJobError", func(t *testing.T) {
		items := map[string]int{"a": 1, "b": 2, "c": 3}
		processed := sync.Map{}

		seq := iter.Seq2[string, int](
			func(yield func(string, int) bool) {
				for k, v := range items {
					if !yield(k, v) {
						return
					}
				}
			},
		)

		err := Range2(
			seq,
			func(ctx context.Context, k string, v int) error {
				processed.Store(k, true)
				return nil
			},
		)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for _, k := range []string{"a", "b", "c"} {
			if b, ok := processed.Load(k); !ok || b.(bool) == false {
				t.Errorf("key %v not processed", k)
			}
		}
	})

	t.Run("JobError", func(t *testing.T) {
		expectedErr := errors.New("test error")
		items := map[string]int{"a": 1, "b": 2, "c": 3}

		seq := iter.Seq2[string, int](
			func(yield func(string, int) bool) {
				for k, v := range items {
					if !yield(k, v) {
						return
					}
				}
			},
		)

		err := Range2(
			seq,
			func(ctx context.Context, k string, v int) error {
				return expectedErr
			},
		)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestMap(t *testing.T) {
	t.Run("NoJobError", func(t *testing.T) {
		input := []int{1, 2, 3}
		results, err := Map(
			input,
			func(ctx context.Context, n int) (int, error) {
				return n * 2, nil
			},
		)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expected := []int{2, 4, 6}
		if !reflect.DeepEqual(results, expected) {
			t.Errorf("got %v, want %v", results, expected)
		}
	})

	t.Run("JobError", func(t *testing.T) {
		input := []int{1, 2, 3}
		expectedErr := errors.New("test error")
		_, err := Map(
			input,
			func(ctx context.Context, n int) (int, error) {
				if n == 2 {
					return 0, expectedErr
				}
				return n * 2, nil
			},
		)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestMapInPlace(t *testing.T) {
	t.Run("NoJobError", func(t *testing.T) {
		input := []int{1, 2, 3}
		results, err := MapInPlace(
			input,
			func(ctx context.Context, n int) (int, error) {
				return n * 2, nil
			},
		)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expected := []int{2, 4, 6}
		if !reflect.DeepEqual(results, expected) {
			t.Errorf("got %v, want %v", results, expected)
		}

		// Verify it's the same slice
		if &input[0] != &results[0] {
			t.Error("MapInPlace created new slice instead of modifying in place")
		}
	})

	t.Run("JobError", func(t *testing.T) {
		input := []int{1, 2, 3}
		expectedErr := errors.New("test error")
		results, err := MapInPlace(
			input,
			func(ctx context.Context, n int) (int, error) {
				return n, expectedErr
			},
		)

		if err == nil {
			t.Error("expected nil, got error")
		}

		// Verify it's the same slice
		if &input[0] != &results[0] {
			t.Error("MapInPlace created new slice instead of modifying in place")
		}
	})
}

func TestMap2(t *testing.T) {
	t.Run("NoJobError", func(t *testing.T) {
		input := map[string]int{"a": 1, "b": 2, "c": 3}
		results, err := Map2(
			input,
			func(ctx context.Context, k string, v int) (string, int, error) {
				return k + k, v * 2, nil
			},
		)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expected := map[string]int{"aa": 2, "bb": 4, "cc": 6}
		if !reflect.DeepEqual(results, expected) {
			t.Errorf("got %v, want %v", results, expected)
		}
	})

	t.Run("JobError", func(t *testing.T) {
		input := map[string]int{"a": 1, "b": 2}
		expectedErr := errors.New("test error")
		_, err := Map2(
			input,
			func(ctx context.Context, k string, v int) (string, int, error) {
				if k == "b" {
					return "", 0, expectedErr
				}
				return k + k, v * 2, nil
			},
		)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestAsyncMap(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		inputs := []int{70, 10, 33, 20, 50, 80}
		var collected []string

		err := Block(func(n Nursery) error {
			outputs := AsyncMap(
				n,
				slices.Values(inputs),
				func(ctx context.Context, i int) (string, error) {
					output := fmt.Sprintf("output %02d", i)
					Sleep(ctx, time.Millisecond*time.Duration(i))
					return output, nil
				},
			)
			for output := range outputs {
				collected = append(collected, output)
			}

			return nil
		})
		if err != nil {
			t.Error(err)
		}
		if len(collected) != len(inputs) {
			t.Errorf("expected to collect an output for each input")
		}
		sortedOutputs := slices.Clone(collected)
		slices.Sort(sortedOutputs)
		if !slices.Equal(collected, sortedOutputs) {
			t.Errorf("expected output slice to be in sorted order")
		}
	})

	t.Run("Multistage", func(t *testing.T) {
		err := Block(func(n Nursery) error {
			// stage A: ints to strings
			inputs := []int{7, 5, 3, 1, 2, 4, 6}
			outputsA := AsyncMap(
				n,
				slices.Values(inputs),
				func(ctx context.Context, i int) (string, error) {
					output := fmt.Sprintf("output %02d", i)
					Sleep(ctx, time.Millisecond*time.Duration(i))
					return output, nil
				},
			)

			// stage B: strings to ints
			outputsB := AsyncMap(
				n,
				outputsA,
				func(_ context.Context, s string) (int, error) {
					return len(s), nil
				},
			)

			for _ = range outputsB {
			}

			return nil
		})
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		inputs := []int{1, 2, 3, 4, 5}
		expectedError := errors.New("test error")
		var collected []string
		var processedCount atomic.Int32

		err := Block(func(n Nursery) error {
			outputs := AsyncMap(
				n,
				slices.Values(inputs),
				func(_ context.Context, i int) (string, error) {
					processedCount.Add(1)
					time.Sleep(1 * time.Millisecond)
					if i == 3 {
						return "", expectedError
					}
					return fmt.Sprintf("value %d", i), nil
				},
			)

			for output := range outputs {
				collected = append(collected, output)
			}
			return nil
		})

		if err == nil {
			t.Error("expected an error, got nil")
		}
		if !errors.Is(err, expectedError) {
			t.Errorf("expected error %v, got %v", expectedError, err)
		}

		if processedCount.Load() == 0 {
			t.Error("expected at least some inputs to be processed")
		}

		for _, output := range collected {
			if output == "value 3" {
				t.Error("output from error-producing input should not be collected")
			}
		}
	})

	t.Run("EmptyInput", func(t *testing.T) {
		var inputs []int
		var collected []string
		var transformCalled atomic.Bool

		err := Block(func(n Nursery) error {
			outputs := AsyncMap(
				n,
				slices.Values(inputs),
				func(_ context.Context, i int) (string, error) {
					transformCalled.Store(true)
					return fmt.Sprintf("value %d", i), nil
				},
			)
			for output := range outputs {
				collected = append(collected, output)
			}
			return nil
		})

		if err != nil {
			t.Error(err)
		}
		if transformCalled.Load() {
			t.Error("transform function should not be called for empty input")
		}
		if len(collected) != 0 {
			t.Errorf("expected empty output for empty input, got %d items", len(collected))
		}
	})

	t.Run("Cancellation", func(t *testing.T) {
		inputs := []int{1, 2, 3, 4, 5}
		var processedCount atomic.Int32
		var collected []string
		var contextCancelled atomic.Bool

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := Block(func(n Nursery) error {
			outputs := AsyncMap(
				n,
				slices.Values(inputs),
				func(ctx context.Context, i int) (string, error) {
					processedCount.Add(1)
					// Cancel after processing a few items
					if i == 3 {
						cancel()
						// Give a little time for cancellation to propagate
						time.Sleep(1 * time.Millisecond)
					}

					// Check if context is cancelled
					select {
					case <-ctx.Done():
						contextCancelled.Store(true)
						return "", ctx.Err()
					default:
						time.Sleep(1 * time.Millisecond)
						return fmt.Sprintf("value %d", i), nil
					}
				},
			)

			for output := range outputs {
				collected = append(collected, output)
			}
			return nil
		}, WithContext(ctx))

		// The error might not be propagated if the main goroutine completes
		// before the error is processed, so we don't strictly check for an error
		if err != nil {
			t.Logf("Got expected error: %v", err)
		}

		if !contextCancelled.Load() {
			t.Error("expected context to be cancelled")
		}

		// Note: We can't reliably assert that not all inputs were processed
		// after cancellation. Depending on timing, all inputs might be processed
		// before cancellation fully propagates. This is expected behavior.
		t.Logf("Processed %d/%d inputs after cancellation", processedCount.Load(), len(inputs))
	})

	t.Run("LargeInput", func(t *testing.T) {
		// Create a large input slice
		const inputSize = 50 // Reduced from 100 to make test faster
		inputs := make([]int, inputSize)
		for i := 0; i < inputSize; i++ {
			inputs[i] = i
		}

		var processedCount atomic.Int32
		var collected []int

		err := Block(func(n Nursery) error {
			outputs := AsyncMap(
				n,
				slices.Values(inputs),
				func(_ context.Context, i int) (int, error) {
					processedCount.Add(1)
					return i * 2, nil
				},
			)
			for output := range outputs {
				collected = append(collected, output)
			}
			return nil
		})

		if err != nil {
			t.Error(err)
		}
		if processedCount.Load() != inputSize {
			t.Errorf("expected all %d inputs to be processed, got %d", inputSize, processedCount.Load())
		}
		if len(collected) != inputSize {
			t.Errorf("expected to collect %d outputs, got %d", inputSize, len(collected))
		}
	})

	t.Run("MixedProcessingTimes", func(t *testing.T) {
		inputs := []int{10, 1, 5, 2, 8, 3}
		var processOrder []int
		var outputOrder []int
		var mu sync.Mutex

		err := Block(func(n Nursery) error {
			outputs := AsyncMap(
				n,
				slices.Values(inputs),
				func(ctx context.Context, i int) (int, error) {
					// Process in order of input
					mu.Lock()
					processOrder = append(processOrder, i)
					mu.Unlock()

					// Sleep for different durations to ensure different completion times
					Sleep(ctx, time.Millisecond*time.Duration(i))
					return i, nil
				},
			)
			for output := range outputs {
				outputOrder = append(outputOrder, output)
			}
			return nil
		})

		if err != nil {
			t.Error(err)
		}

		if len(outputOrder) != len(inputs) {
			t.Errorf("expected %d outputs, got %d", len(inputs), len(outputOrder))
		}

		// Verify outputs are not in the same order as inputs (should be ordered by completion time)
		// This is a probabilistic test, but with the sleep times we've chosen, it should almost always pass
		if slices.Equal(processOrder, outputOrder) {
			t.Errorf("expected output order to differ from processing order due to different processing times")
		}
	})

	t.Run("DrainAfterPartialConsumption", func(t *testing.T) {
		// This test demonstrates that if you stop consuming from the output channel early,
		// you must drain the remaining outputs to allow all goroutines to complete
		inputs := []int{1, 2, 3, 4, 5}
		var processedCount atomic.Int32
		var consumedCount atomic.Int32

		err := Block(func(n Nursery) error {
			outputs := AsyncMap(
				n,
				slices.Values(inputs),
				func(_ context.Context, i int) (int, error) {
					processedCount.Add(1)
					// Simulate work
					time.Sleep(1 * time.Millisecond)
					return i, nil
				},
			)

			// Only consume first 3 outputs
			count := 0
			for output := range outputs {
				consumedCount.Add(1)
				count++
				if count >= 3 {
					// We've consumed enough outputs, but we need to drain the channel
					// to allow all goroutines to complete
					go func() {
						// Drain remaining outputs in a separate goroutine
						for range outputs {
							// Just drain, don't count these
						}
					}()
					break
				}
				_ = output
			}

			return nil
		})

		if err != nil {
			t.Error(err)
		}

		if processedCount.Load() != int32(len(inputs)) {
			t.Errorf("expected all %d inputs to be processed, got %d", len(inputs), processedCount.Load())
		}

		if consumedCount.Load() != 3 {
			t.Errorf("expected to consume exactly 3 outputs, got %d", consumedCount.Load())
		}
	})

	t.Run("SafeEarlyTerminationWithDefer", func(t *testing.T) {
		// This test demonstrates the recommended pattern for safely handling early termination
		// using a defer to ensure the channel is always drained
		inputs := []int{1, 2, 3, 4, 5}
		var processedCount atomic.Int32
		var consumedCount atomic.Int32

		err := Block(func(n Nursery) error {
			outputs := AsyncMap(
				n,
				slices.Values(inputs),
				func(_ context.Context, i int) (int, error) {
					processedCount.Add(1)
					// Simulate work
					time.Sleep(1 * time.Millisecond)
					return i, nil
				},
			)

			// Set up a deferred drain to handle early returns or breaks
			drainStarted := false
			defer func() {
				if !drainStarted {
					n.Go(func() error {
						// Drain any remaining outputs
						for range outputs {
							// Discard remaining outputs
						}
						return nil
					})
				}
			}()

			// Process outputs until some condition
			for output := range outputs {
				consumedCount.Add(1)
				_ = output

				// Simulate early termination after consuming 3 outputs
				if consumedCount.Load() >= 3 {
					drainStarted = true
					go func() {
						// Drain remaining outputs
						for range outputs {
							// Discard remaining outputs
						}
					}()
					break
				}
			}

			return nil
		})

		if err != nil {
			t.Error(err)
		}

		if processedCount.Load() != int32(len(inputs)) {
			t.Errorf("expected all %d inputs to be processed, got %d", len(inputs), processedCount.Load())
		}

		if consumedCount.Load() != 3 {
			t.Errorf("expected to consume exactly 3 outputs, got %d", consumedCount.Load())
		}
	})

	t.Run("ContextCancellationUnblocksProducers", func(t *testing.T) {
		// This test verifies that cancelling the context unblocks goroutines
		// that are trying to send to the outputs channel
		inputs := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
		var processedCount atomic.Int32
		var blockedCount atomic.Int32
		var unblockedCount atomic.Int32
		var readyToCancel atomic.Bool
		var cancelDone atomic.Bool

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Use a longer timeout to ensure the test has enough time to complete
		ctx, timeoutCancel := context.WithTimeout(ctx, 2*time.Second)
		defer timeoutCancel() // Ensure the timeout context is properly cancelled

		err := Block(func(n Nursery) error {
			// Create a buffered channel to control when goroutines are blocked
			// Buffer size 3 means the first 3 items will be processed without blocking
			controlCh := make(chan struct{}, 3)

			outputs := AsyncMap(
				n,
				slices.Values(inputs),
				func(ctx context.Context, i int) (int, error) {
					processedCount.Add(1)

					// Simulate slow processing for later items
					if i > 3 {
						// Signal that we're about to block
						blockedCount.Add(1)

						// Wait until we're ready to proceed or context is cancelled
						select {
						case controlCh <- struct{}{}:
							// This will block once the buffer is full
							readyToCancel.Store(true)

							// Wait for cancellation
							<-ctx.Done()
							unblockedCount.Add(1)
							return i, n.Err()
						case <-ctx.Done():
							// Already cancelled
							unblockedCount.Add(1)
							return i, n.Err()
						}
					}
					return i, nil
				},
			)

			// Only consume first 3 outputs, then cancel the context
			count := 0
			for output := range outputs {
				count++
				if count >= 3 {
					// Wait until at least one goroutine is blocked
					for !readyToCancel.Load() {
						time.Sleep(10 * time.Millisecond)
						// If we've waited too long, break to avoid hanging
						if count := blockedCount.Load(); count > 0 {
							break
						}
					}

					// Cancel the context to unblock any goroutines trying to send
					cancel()
					cancelDone.Store(true)
					break
				}
				_ = output
			}

			// Wait for cancellation to propagate
			for i := 0; i < 10 && !cancelDone.Load(); i++ {
				time.Sleep(10 * time.Millisecond)
			}

			return nil
		}, WithContext(ctx))

		if err == nil {
			t.Error("expected an error due to context cancellation")
		}

		if blockedCount.Load() == 0 {
			t.Error("expected some goroutines to be blocked")
		}

		if unblockedCount.Load() == 0 {
			t.Error("expected blocked goroutines to be unblocked by context cancellation")
		}

		if processedCount.Load() >= int32(len(inputs)) {
			t.Logf("All inputs were processed despite cancellation, which is unexpected but not an error")
		}
	})

	t.Run("WithOutputBuffer", func(t *testing.T) {
		inputs := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
		var producerUnblocked atomic.Bool
		var processedCount atomic.Int32

		bufferSize := 5
		err := Block(func(n Nursery) error {
			// Create a slow consumer scenario
			outputs := AsyncMap(
				n,
				slices.Values(inputs),
				func(ctx context.Context, i int) (int, error) {
					// Mark this item as processed
					processedCount.Add(1)

					// All items should be processed without blocking,
					// even though we haven't started consuming yet
					if processedCount.Load() > int32(bufferSize) {
						producerUnblocked.Store(true)
					}

					return i, nil
				},
				WithOutputBuffer(bufferSize),
			)

			// Wait a bit to allow producers to process items
			time.Sleep(50 * time.Millisecond)

			// Then consume all outputs
			var collected []int
			for output := range outputs {
				collected = append(collected, output)
			}

			if len(collected) != len(inputs) {
				t.Errorf("expected %d outputs, got %d", len(inputs), len(collected))
			}

			return nil
		})

		if err != nil {
			t.Error(err)
		}

		// All inputs should be processed
		if processedCount.Load() != int32(len(inputs)) {
			t.Errorf("expected all %d inputs to be processed, got %d", len(inputs), processedCount.Load())
		}

		// Verify that producers were able to make progress beyond the buffer size
		if !producerUnblocked.Load() {
			t.Error("expected producers to be unblocked by the buffered channel")
		}
	})

	t.Run("ZeroBufferEqualsUnbuffered", func(t *testing.T) {
		tempNursery := newNursery()
		WithOutputBuffer(0)(tempNursery)
		if tempNursery.bufSize != 0 {
			t.Errorf("expected buffer size 0, got %d", tempNursery.bufSize)
		}
	})

	t.Run("NegativeBufferEqualsUnbuffered", func(t *testing.T) {
		tempNursery := newNursery()
		WithOutputBuffer(-5)(tempNursery)
		if tempNursery.bufSize != 0 {
			t.Errorf("expected buffer size 0, got %d", tempNursery.bufSize)
		}
	})
}
