package conc

import (
	"context"
	"errors"
	"iter"
	"reflect"
	"sync"
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
