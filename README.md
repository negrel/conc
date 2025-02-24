![structured concurrency schema](https://external-content.duckduckgo.com/iu/?u=https%3A%2F%2Fwww.thedevtavern.com%2Fstatic%2F671edc5ec17e1522393f07c0f7b42465%2F9bec7%2Fbanner.png&f=1&nofb=1&ipt=3295434f2f3b02cfff98cc60776bf62cf519268dc90982d7d98a3caff5544dce&ipo=images)

# `conc` - Structured concurrency for Go

[![Go doc](https://pkg.go.dev/badge/github.com/negrel/conc)](https://pkg.go.dev/github.com/negrel/conc)
[![go report card](https://goreportcard.com/badge/github.com/negrel/conc)](https://goreportcard.com/report/github.com/negrel/conc)
[![license card](https://img.shields.io/github/license/negrel/conc)](./LICENSE)
[![PRs welcome card](https://img.shields.io/badge/PRs-Welcome-brightgreen)](https://github.com/negrel/conc/pulls)
![Go version card](https://img.shields.io/github/go-mod/go-version/negrel/conc)

`conc` is a **structured concurrency** library for Go that provides a safer,
more intuitive approach to concurrent programming.

By emphasizing proper resource management, error handling, and execution flow,
`conc` helps developers write concurrent code that is less error-prone, easier
to reason about, and aligned with established best practices.

```sh
go get github.com/negrel/conc
```

## Predictable code flow

`conc` is based on `nursery` as described in [this blog post](https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/).


Idea behind nursery is that routines are scoped to a block that returns when
all goroutines are done. This way, code flow remains sequential outside before
and after the block. Here is an example:

```go
func main() {
	conc.Block(func(n conc.Nursery) error {
		// Spawn a goroutine.
		n.Go(func() error {
			return nil
		})

		// Spawn another goroutine.
		n.Go(func() error {
			time.Sleep(2 * time.Second)
			return nil
		})

		// Sleep before returning.
		time.Sleep(time.Second)
		return nil
	})
	// Once block returns (here after 2 seconds), you're guaranteed that there is no
	// goroutine leak.
	// ...
}
```

Here is the definition of the `Nursery` interface:

```go
type Nursery interface {
	context.Context
	Go(func() error)
}
```

It is a simple extension to [`context.Context`](https://pkg.go.dev/context#Context) that allows spawning routines.

[`Block`](https://pkg.go.dev/github.com/negrel/conc#Block) and
[`Nursery`](https://pkg.go.dev/github.com/negrel/conc#Nursery) are the core of
the entire library.

## Explicit goroutines "leak"

Now, let's say you want to write a function spawning routines that outlives it.
You can pass `Nursery` as a parameter making the leak explicit:

```go
func workHard(n conc.Nursery) {
	n.Go(func() error {
		return longWork(n)
	})

	n.Go(func() error {
		return longerWork(n)
	})
}
```

## And more...

* Graceful panics and errors handling
* Context integration for deadlines and cancellation
* Limit maximum number of goroutine used by a block
* Goroutines are pool allocated to improve efficiency
* Dependency free

## Performance

Here, an operation means spawning 100 goroutines:

```sh
$ cd bench/
$ go test -v -bench=./... -benchmem ./...
goos: linux
goarch: amd64
pkg: github.com/negrel/conc/bench
cpu: AMD Ryzen 7 7840U w/ Radeon  780M Graphics
BenchmarkNursery
BenchmarkNursery/EmptyBlock
BenchmarkNursery/EmptyBlock-16   1367210               914.3 ns/op           625 B/op         11 allocs/op
BenchmarkNursery/WithRoutines/NoWork
BenchmarkNursery/WithRoutines/NoWork-16                    26823             43909 ns/op            1514 B/op         65 allocs/op
BenchmarkNursery/WithRoutines/Nested/NoWork
BenchmarkNursery/WithRoutines/Nested/NoWork-16             18189             69416 ns/op            3623 B/op        147 allocs/op
BenchmarkNursery/WithRoutines/1msWork
BenchmarkNursery/WithRoutines/1msWork-16                     940           1300306 ns/op           11941 B/op        211 allocs/op
BenchmarkNursery/WithRoutines/1-10msWork
BenchmarkNursery/WithRoutines/1-10msWork-16                  123           9681105 ns/op           12427 B/op        292 allocs/op
BenchmarkNursery/WithRoutines/Error
BenchmarkNursery/WithRoutines/Error-16                     26767             44257 ns/op            1485 B/op         65 allocs/op
BenchmarkSourceGraphConc
BenchmarkSourceGraphConc/EmptyPool
BenchmarkSourceGraphConc/EmptyPool-16                   13450243                85.45 ns/op          176 B/op          2 allocs/op
BenchmarkSourceGraphConc/WithRoutines/NoWork
BenchmarkSourceGraphConc/WithRoutines/NoWork-16            48522             23928 ns/op            1835 B/op         84 allocs/op
BenchmarkSourceGraphConc/WithRoutines/1msWork
BenchmarkSourceGraphConc/WithRoutines/1msWork-16                     966           1239140 ns/op           13776 B/op        302 allocs/op
BenchmarkSourceGraphConc/WithRoutines/1-10msWork
BenchmarkSourceGraphConc/WithRoutines/1-10msWork-16                  123           9625635 ns/op           14030 B/op        372 allocs/op
BenchmarkGo
BenchmarkGo/WithRoutines/NoWork
BenchmarkGo/WithRoutines/NoWork-16                                 84024             14240 ns/op            1600 B/op        100 allocs/op
BenchmarkGo/WithRoutines/Nested/NoWork
BenchmarkGo/WithRoutines/Nested/NoWork-16                          81360             14497 ns/op            3199 B/op        199 allocs/op
BenchmarkGo/WithRoutines/1msWork
BenchmarkGo/WithRoutines/1msWork-16                                59904             18651 ns/op           11215 B/op        200 allocs/op
BenchmarkGo/WithRoutines/1-10msWork
BenchmarkGo/WithRoutines/1-10msWork-16                             51703             21088 ns/op           11069 B/op        190 allocs/op
PASS
ok      github.com/negrel/conc/bench    22.347s
```

## Contributing

If you want to contribute to `conc` to add a feature or improve the code contact
me at [alexandre@negrel.dev](mailto:alexandre@negrel.dev), open an
[issue](https://github.com/negrel/conc/issues) or make a
[pull request](https://github.com/negrel/conc/pulls).

## :stars: Show your support

Please give a :star: if this project helped you!

[![buy me a coffee](https://github.com/negrel/.github/blob/master/.github/images/bmc-button.png?raw=true)](https://www.buymeacoffee.com/negrel)

## :scroll: License

MIT © [Alexandre Negrel](https://www.negrel.dev/)
