package conc

import "unsafe"

type joinError struct {
	errs []error
}

func joinErrors(je **joinError, err error) {
	if *je == nil {
		*je = &joinError{}
	}

	(*je).errs = append((*je).errs, err)
}

// Error implements error.
func (e *joinError) Error() string {
	// Copied from std errors package.

	// Since Join returns nil if every value in errs is nil,
	// e.errs cannot be empty.
	if len(e.errs) == 1 {
		return e.errs[0].Error()
	}

	b := []byte(e.errs[0].Error())
	for _, err := range e.errs[1:] {
		b = append(b, '\n')
		b = append(b, err.Error()...)
	}
	// At this point, b has at least one byte '\n'.
	return unsafe.String(&b[0], len(b))
}

// Copied from std errors.
func (e *joinError) Unwrap() []error {
	return e.errs
}
