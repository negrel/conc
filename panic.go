package conc

import (
	"fmt"
)

// GoroutinePanic holds value from a recovered panic along a stacktrace.
type GoroutinePanic struct {
	Value any
	Stack string
}

// String implements fmt.Stringer.
func (gp GoroutinePanic) String() string {
	return fmt.Sprintf("%v\n%v", gp.Value, gp.Stack)
}

// Error implements error.
func (gp GoroutinePanic) Error() string {
	return gp.String()
}

// Unwrap unwrap underlying error.
func (gp GoroutinePanic) Unwrap() error {
	if err, isErr := gp.Value.(error); isErr {
		return err
	}

	return nil
}
