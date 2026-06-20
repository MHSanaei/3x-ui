// Package common provides common utility functions for error handling, formatting, and multi-error management.
package common

import (
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
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

// GoRecover runs fn in a new goroutine guarded by a recover, so a panic in a
// background goroutine is logged (with name and a stack trace) instead of taking
// the whole process down. name identifies the goroutine in the log.
func GoRecover(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic in goroutine", name, ":", r, "\n"+string(debug.Stack()))
			}
		}()
		fn()
	}()
}
