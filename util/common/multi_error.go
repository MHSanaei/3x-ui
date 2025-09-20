package common

import (
	"strings"
)

// multiError represents a collection of errors.
type multiError []error

// Error returns a string representation of all errors joined with " | ".
func (e multiError) Error() string {
	var r strings.Builder
	r.WriteString("multierr: ")
	for _, err := range e {
		r.WriteString(err.Error())
		r.WriteString(" | ")
	}
	return r.String()
}

// Combine combines multiple errors into a single error, filtering out nil errors.
func Combine(maybeError ...error) error {
	var errs multiError
	for _, err := range maybeError {
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
