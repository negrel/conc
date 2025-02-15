module github.com/negrel/conc/bench

go 1.23.4

replace github.com/negrel/conc => ..

require (
	github.com/negrel/conc v0.0.0-00010101000000-000000000000
	github.com/sourcegraph/conc v0.3.0
)

require (
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
)
