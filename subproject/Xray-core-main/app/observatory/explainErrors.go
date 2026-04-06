package observatory

import "github.com/xtls/xray-core/common/errors"

type errorCollector struct {
	errors *errors.Error
}

func (e *errorCollector) SubmitError(err error) {
	if e.errors == nil {
		e.errors = errors.New("underlying connection error").Base(err)
		return
	}
	e.errors = e.errors.Base(errors.New("underlying connection error").Base(err))
}

func newErrorCollector() *errorCollector {
	return &errorCollector{}
}

func (e *errorCollector) UnderlyingError() error {
	if e.errors == nil {
		return errors.New("failed to produce report")
	}
	return e.errors
}
