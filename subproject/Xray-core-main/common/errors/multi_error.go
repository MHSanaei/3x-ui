package errors

import (
	"errors"
	"strings"
)

type multiError []error

func (e multiError) Error() string {
	var r strings.Builder
	r.WriteString("multierr: ")
	for _, err := range e {
		r.WriteString(err.Error())
		r.WriteString(" | ")
	}
	return r.String()
}

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

func AllEqual(expected error, actual error) bool {
	switch errs := actual.(type) {
	case multiError:
		if len(errs) == 0 {
			return false
		}
		for _, err := range errs {
			if !errors.Is(err, expected) {
				return false
			}
		}
		return true
	default:
		return errors.Is(errs, expected)
	}
}
