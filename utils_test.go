package conc

import (
	"context"
	"errors"
	"iter"
	"maps"
	"reflect"
	"testing"
	"time"
)

func TestSleep(t *testing.T) {
	t.Run("respects duration", func(t *testing.T) {
		ctx := context.Background()
		start := time.Now()
		Sleep(ctx, 100*time.Millisecond)
		elapsed := time.Since(start)
		if elapsed < 100*time.Millisecond {
			t.Errorf("Sleep returned too early: got %v, want >= 100ms", elapsed)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
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

func TestAll(t *testing.T) {
	t.Run("executes all jobs", func(t *testing.T) {
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

	t.Run("handles errors", func(t *testing.T) {
		expectedErr := errors.New("test error")
		jobs := []Job[int]{
			func(ctx context.Context) (int, error) { return 1, nil },
			func(ctx context.Context) (int, error) { return 0, expectedErr },
			func(ctx context.Context) (int, error) { return 3, nil },
		}

		_, err := All(jobs)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestRace(t *testing.T) {
	t.Run("returns first result", func(t *testing.T) {
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

	t.Run("handles errors", func(t *testing.T) {
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
	t.Run("processes all items", func(t *testing.T) {
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

	t.Run("handles errors", func(t *testing.T) {
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
	t.Run("processes all items", func(t *testing.T) {
		items := map[string]int{"a": 1, "b": 2, "c": 3}
		processed := make(map[string]bool)

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
				processed[k] = true
				return nil
			},
		)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expected := map[string]bool{"a": true, "b": true, "c": true}
		if !reflect.DeepEqual(processed, expected) {
			t.Errorf("got %v, want %v", processed, expected)
		}
	})
}

func TestMap(t *testing.T) {
	t.Run("maps all items", func(t *testing.T) {
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

	t.Run("handles errors", func(t *testing.T) {
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
	t.Run("maps all items in place", func(t *testing.T) {
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
}

func TestMap2(t *testing.T) {
	t.Run("maps all items", func(t *testing.T) {
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

	t.Run("handles errors", func(t *testing.T) {
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

func TestMap2InPlace(t *testing.T) {
	t.Run("maps all items in place", func(t *testing.T) {
		input := map[string]int{"a": 1, "b": 2, "c": 3}
		originalInput := maps.Clone(input)

		results, err := Map2InPlace(
			input,
			func(ctx context.Context, k string, v int) (string, int, error) {
				return k, v * 2, nil
			},
		)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expected := map[string]int{"a": 2, "b": 4, "c": 6}
		if !reflect.DeepEqual(results, expected) {
			t.Errorf("got %v, want %v", results, expected)
		}

		// Verify the input map was modified
		if reflect.DeepEqual(input, originalInput) {
			t.Error("Map2InPlace did not modify the input map")
		}
		if !reflect.DeepEqual(input, expected) {
			t.Error("Map2InPlace did not correctly modify the input map")
		}
	})
}
