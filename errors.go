package neta

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidConfig indicates that an effective Client, Server or Session configuration is invalid.
	ErrInvalidConfig = errors.New("neta: invalid configuration")
	// ErrNotBound indicates that an operation requires a Bound Session.
	ErrNotBound = errors.New("neta: session is not bound")
	// ErrClosing indicates that the Session is closing and no longer accepts new Application work.
	ErrClosing = errors.New("neta: session is closing")
	// ErrClosed indicates that the Client, Server or Session is terminally closed.
	ErrClosed = errors.New("neta: session is closed")
	// ErrQueueFull indicates that the per-Session outgoing Application FIFO cannot admit the frame.
	ErrQueueFull = errors.New("neta: application queue is full")
	// ErrCapacity indicates that a configured resource or admission budget is exhausted.
	ErrCapacity = errors.New("neta: resource capacity exhausted")
	// ErrFrameTooLarge indicates that a Frame exceeds the fixed neta-cpp-compatible 1 MiB total size.
	ErrFrameTooLarge = errors.New("neta: frame exceeds the protocol maximum")
	// ErrRequestTimeout indicates that an admitted Request did not receive its matching Response in time.
	ErrRequestTimeout = errors.New("neta: request timed out")
	// ErrConnectTimeout indicates that a Client Dial attempt exceeded ConnectTimeout.
	ErrConnectTimeout = errors.New("neta: connect timed out")
	// ErrBindTimeout indicates that the Client Bind handshake exceeded BindResponseTimeout.
	ErrBindTimeout = errors.New("neta: bind timed out")
	// ErrWriteTimeout indicates that writing a Frame exceeded WriteTimeout.
	ErrWriteTimeout = errors.New("neta: frame write timed out")
	// ErrUnbindTimeout indicates that graceful Unbind exceeded its approved deadline.
	ErrUnbindTimeout = errors.New("neta: unbind timed out")
	// ErrBindRejected indicates that the peer rejected the Bind Request.
	ErrBindRejected = errors.New("neta: bind rejected")
	// ErrProtocol indicates a malformed or directionally invalid protocol operation.
	ErrProtocol = errors.New("neta: protocol error")
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

// Error returns the diagnostic configuration error string.
func (e *ConfigError) Error() string {
	if e == nil {
		return ErrInvalidConfig.Error()
	}
	return fmt.Sprintf("%v: %s=%q; %s", ErrInvalidConfig, e.field, e.value, e.constraint)
}

// Unwrap returns ErrInvalidConfig for errors.Is and errors.As inspection.
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

// Error returns the operation and underlying cause.
func (e *OperationError) Error() string {
	if e == nil {
		return ""
	}
	if e.cause == nil {
		return e.operation
	}
	return fmt.Sprintf("%s: %v", e.operation, e.cause)
}

// Unwrap returns the underlying cause for errors.Is/errors.As.
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
