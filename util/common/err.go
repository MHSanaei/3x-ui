// Package common provides common utility functions for error handling, formatting, and multi-error management.
package common

import (
	"errors"
	"fmt"

	"github.com/mhsanaei/3x-ui/v2/logger"
)

// NewErrorf creates a new error with formatted message.
func NewErrorf(format string, a ...any) error {
	msg := fmt.Sprintf(format, a...)
	return errors.New(msg)
}

// NewError creates a new error from the given arguments.
func NewError(a ...any) error {
	msg := fmt.Sprintln(a...)
	return errors.New(msg)
}

// Recover handles panic recovery and logs the panic error if a message is provided.
func Recover(msg string) any {
	panicErr := recover()
	if panicErr != nil {
		if msg != "" {
			logger.Error(msg, "panic:", panicErr)
		}
	}
	return panicErr
}
