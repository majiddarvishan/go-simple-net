package neta

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidConfig  = errors.New("neta: invalid configuration")
	ErrNotBound       = errors.New("neta: session is not bound")
	ErrClosing        = errors.New("neta: session is closing")
	ErrClosed         = errors.New("neta: session is closed")
	ErrQueueFull      = errors.New("neta: application queue is full")
	ErrCapacity       = errors.New("neta: resource capacity exhausted")
	ErrFrameTooLarge  = errors.New("neta: frame exceeds the protocol maximum")
	ErrRequestTimeout = errors.New("neta: request timed out")
	ErrConnectTimeout = errors.New("neta: connect timed out")
	ErrBindTimeout    = errors.New("neta: bind timed out")
	ErrWriteTimeout   = errors.New("neta: frame write timed out")
	ErrUnbindTimeout  = errors.New("neta: unbind timed out")
	ErrBindRejected   = errors.New("neta: bind rejected")
	ErrProtocol       = errors.New("neta: protocol error")
)

// ConfigError describes one invalid effective configuration field.
// It unwraps to ErrInvalidConfig, so callers can use errors.Is.
type ConfigError struct {
	field      string
	value      string
	constraint string
}

func newConfigError(field, value, constraint string) *ConfigError {
	return &ConfigError{field: field, value: value, constraint: constraint}
}

func (e *ConfigError) Error() string {
	if e == nil {
		return ErrInvalidConfig.Error()
	}
	return fmt.Sprintf("%v: %s=%q; %s", ErrInvalidConfig, e.field, e.value, e.constraint)
}

func (e *ConfigError) Unwrap() error { return ErrInvalidConfig }

// Field returns the configuration field that failed validation.
func (e *ConfigError) Field() string {
	if e == nil {
		return ""
	}
	return e.field
}

// Value returns the rejected value in its diagnostic representation.
func (e *ConfigError) Value() string {
	if e == nil {
		return ""
	}
	return e.value
}

// Constraint returns the validation rule that was violated.
func (e *ConfigError) Constraint() string {
	if e == nil {
		return ""
	}
	return e.constraint
}

// OperationError adds operation context while preserving errors.Is/errors.As.
type OperationError struct {
	operation string
	cause     error
}

func newOperationError(operation string, cause error) *OperationError {
	return &OperationError{operation: operation, cause: cause}
}

func (e *OperationError) Error() string {
	if e == nil {
		return ""
	}
	if e.cause == nil {
		return e.operation
	}
	return fmt.Sprintf("%s: %v", e.operation, e.cause)
}

func (e *OperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Operation returns the operation name associated with the error.
func (e *OperationError) Operation() string {
	if e == nil {
		return ""
	}
	return e.operation
}
