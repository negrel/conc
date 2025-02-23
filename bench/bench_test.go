package main

import (
	"io"
	"testing"
	"time"

	"github.com/negrel/conc"
	"github.com/sourcegraph/conc/pool"
)

const (
	benchRoutineCount = 100
)

func BenchmarkNursery(b *testing.B) {
	b.Run("EmptyBlock", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			conc.Block(func(n conc.Nursery) error {
				return nil
			})
		}
	})

	b.Run("WithRoutines/NoWork", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			conc.Block(func(n conc.Nursery) error {
				for j := 0; j < benchRoutineCount; j++ {
					n.Go(func() error {
						return nil
					})
				}
				return nil
			})
		}
	})

	b.Run("WithRoutines/Nested/NoWork", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			conc.Block(func(n conc.Nursery) error {
				for j := 0; j < benchRoutineCount; j++ {
					n.Go(func() error {
						n.Go(func() error {
							return nil
						})
						return nil
					})
				}
				return nil
			})
		}
	})

	b.Run("WithRoutines/1msWork", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			conc.Block(func(n conc.Nursery) error {
				for j := 0; j < benchRoutineCount; j++ {
					n.Go(func() error {
						time.Sleep(time.Millisecond)
						return nil
					})
				}
				return nil
			})
		}
	})

	b.Run("WithRoutines/1-10msWork", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			conc.Block(func(n conc.Nursery) error {
				for j := 0; j < benchRoutineCount; j++ {
					k := j
					n.Go(func() error {
						time.Sleep(time.Duration(k%10) * time.Millisecond)
						return nil
					})
				}
				return nil
			})
		}
	})

	b.Run("WithRoutines/Error", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := conc.Block(func(n conc.Nursery) error {
				for j := 0; j < benchRoutineCount; j++ {
					n.Go(func() error {
						return io.EOF
					})
				}
				return nil
			}, conc.WithIgnoreErrors())
			if err != nil {
				b.Fatal("block returned non-nil error")
			}
		}
	})
}

func BenchmarkSourceGraphConc(b *testing.B) {
	b.Run("EmptyPool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var p pool.Pool
			p.Wait()
		}
	})

	b.Run("WithRoutines/NoWork", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var p pool.Pool
			for j := 0; j < benchRoutineCount; j++ {
				p.Go(func() {
				})
			}
			p.Wait()
		}
	})

	// for _, routine := range []int{10 /* 1000, 100000 */} {
	// 	b.Run(fmt.Sprintf("WithRoutines/Nested%d/NoWork", routine), func(b *testing.B) {
	// 		for i := 0; i < b.N; i++ {
	// 			var p pool.Pool
	// 			for j := 0; j < routine; j++ {
	// 				p.Go(func() {
	// 					p.Go(func() {
	// 					})
	// 				})
	// 			}
	// 			p.Wait()
	// 		}
	// 	})
	// }

	b.Run("WithRoutines/1msWork", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var p pool.Pool
			for j := 0; j < benchRoutineCount; j++ {
				p.Go(func() {
					time.Sleep(time.Millisecond)
				})
			}
			p.Wait()
		}
	})

	b.Run("WithRoutines/1-10msWork", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var p pool.Pool
			for j := 0; j < benchRoutineCount; j++ {
				k := j
				p.Go(func() {
					time.Sleep(time.Duration(k%10) * time.Millisecond)
				})
			}
			p.Wait()
		}
	})
}

func BenchmarkGo(b *testing.B) {
	b.Run("WithRoutines/NoWork", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j := 0; j < benchRoutineCount; j++ {
				go func() error {
					return nil
				}()
			}
		}
	})

	b.Run("WithRoutines/Nested/NoWork", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j := 0; j < benchRoutineCount; j++ {
				go func() error {
					go func() error {
						return nil
					}()
					return nil
				}()
			}
		}
	})

	b.Run("WithRoutines/1msWork", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j := 0; j < benchRoutineCount; j++ {
				go func() error {
					time.Sleep(time.Millisecond)
					return nil
				}()
			}
		}
	})

	b.Run("WithRoutines/1-10msWork", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j := 0; j < benchRoutineCount; j++ {
				go func(k int) {
					time.Sleep(time.Duration(k%10) * time.Millisecond)
				}(j)
			}
		}
	})
}
