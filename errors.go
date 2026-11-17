package neta

import (
	"errors"
	"fmt"
)

// Error sentinels are stable categories intended for use with errors.Is.
var (
	ErrInvalidConfig  = errors.New("neta: invalid configuration")
	ErrNotBound       = errors.New("neta: session is not bound")
	ErrClosing        = errors.New("neta: session is closing")
	ErrClosed         = errors.New("neta: session is closed")
	ErrQueueFull      = errors.New("neta: queue is full")
	ErrCapacity       = errors.New("neta: resource capacity exhausted")
	ErrFrameTooLarge  = errors.New("neta: frame exceeds protocol maximum")
	ErrRequestTimeout = errors.New("neta: request timed out")
	ErrConnectTimeout = errors.New("neta: connect timed out")
	ErrBindTimeout    = errors.New("neta: bind timed out")
	ErrWriteTimeout   = errors.New("neta: write timed out")
	ErrUnbindTimeout  = errors.New("neta: unbind timed out")
	ErrBindRejected   = errors.New("neta: bind rejected")
	ErrProtocol       = errors.New("neta: protocol error")
)

// ConfigError identifies the setting that made an effective configuration invalid.
// It unwraps to ErrInvalidConfig so callers can use errors.Is.
type ConfigError struct {
	Field  string // Name of the invalid setting.
	Value  any    // Rejected value; identity-bearing values are redacted.
	Reason string // Human-readable validation reason.
}

func (e *ConfigError) Error() string {
	if e == nil {
		return ErrInvalidConfig.Error()
	}
	return fmt.Sprintf("%v: %s=%v: %s", ErrInvalidConfig, e.Field, e.Value, e.Reason)
}

func (e *ConfigError) Unwrap() error { return ErrInvalidConfig }

// OperationError adds operation context while preserving errors.Is/errors.As behavior.
type OperationError struct {
	Op  string // Operation being performed.
	Err error  // Underlying failure.
}

func (e *OperationError) Error() string {
	if e == nil {
		return "neta: operation failed"
	}
	if e.Op == "" {
		return fmt.Sprintf("neta: %v", e.Err)
	}
	return fmt.Sprintf("neta: %s: %v", e.Op, e.Err)
}

func (e *OperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func invalidConfig(field string, value any, reason string) error {
	return &ConfigError{Field: field, Value: value, Reason: reason}
}
