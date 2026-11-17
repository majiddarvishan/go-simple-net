// Package neta provides a bounded, callback-first TCP messaging client and
// server compatible with the Pinex wire protocol.
//
// Client and Server configuration is supplied through typed Functional
// Options. Constructors resolve defaults and validate the complete effective
// configuration before any network or goroutine resources are created.
package neta
