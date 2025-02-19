package main

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/negrel/conc"
	"github.com/sourcegraph/conc/pool"
)

func BenchmarkNursery(b *testing.B) {
	b.Run("EmptyBlock", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			conc.Block(func(n conc.Nursery) error {
				return nil
			})
		}
	})

	for _, routine := range []int{10, 1000, 100000} {
		b.Run(fmt.Sprintf("WithRoutines/%d/NoWork", routine), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				conc.Block(func(n conc.Nursery) error {
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

	for _, routine := range []int{10, 1000, 100000} {
		b.Run(fmt.Sprintf("WithRoutines/%d/1msWork", routine), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				conc.Block(func(n conc.Nursery) error {
					for j := 0; j < routine; j++ {
						n.Go(func() error {
							time.Sleep(time.Millisecond)
							return nil
						})
					}
					return nil
				})
			}
		})
	}

	for _, routine := range []int{10, 1000, 100000} {
		b.Run(fmt.Sprintf("WithRoutines/%d/1-10msWork", routine), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				conc.Block(func(n conc.Nursery) error {
					for j := 0; j < routine; j++ {
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
	}

	for _, routine := range []int{10, 1000, 100000} {
		b.Run(fmt.Sprintf("WithRoutines/%d/Error", routine), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := conc.Block(func(n conc.Nursery) error {
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

func BenchmarkSourceGraphConc(b *testing.B) {
	b.Run("EmptyPool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var p pool.Pool
			p.Wait()
		}
	})

	for _, routine := range []int{10, 1000, 100000} {
		b.Run(fmt.Sprintf("WithRoutines/%d/NoWork", routine), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var p pool.Pool
				for j := 0; j < routine; j++ {
					p.Go(func() {
					})
				}
				p.Wait()
			}
		})
	}

	for _, routine := range []int{10, 1000, 100000} {
		b.Run(fmt.Sprintf("WithRoutines/%d/1msWork", routine), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var p pool.Pool
				for j := 0; j < routine; j++ {
					p.Go(func() {
						time.Sleep(time.Millisecond)
					})
				}
				p.Wait()
			}
		})
	}

	for _, routine := range []int{10, 1000, 100000} {
		b.Run(fmt.Sprintf("WithRoutines/%d/1-10msWork", routine), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var p pool.Pool
				for j := 0; j < routine; j++ {
					k := j
					p.Go(func() {
						time.Sleep(time.Duration(k%10) * time.Millisecond)
					})
				}
				p.Wait()
			}
		})
	}
}

func BenchmarkGo(b *testing.B) {
	for _, routine := range []int{10, 1000, 100000} {
		b.Run(fmt.Sprintf("WithRoutines/%d/NoWork", routine), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for j := 0; j < routine; j++ {
					go func() error {
						return nil
					}()
				}
			}
		})
	}

	for _, routine := range []int{10, 1000, 100000} {
		b.Run(fmt.Sprintf("WithRoutines/%d/1msWork", routine), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for j := 0; j < routine; j++ {
					go func() error {
						time.Sleep(time.Millisecond)
						return nil
					}()
				}
			}
		})
	}

	for _, routine := range []int{10, 1000, 100000} {
		b.Run(fmt.Sprintf("WithRoutines/%d/1-10msWork", routine), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for j := 0; j < routine; j++ {
					go func(k int) {
						time.Sleep(time.Duration(k%10) * time.Millisecond)
					}(j)
				}
			}
		})
	}
}
