package logging

import (
	"io"
	"log"
	"log/slog"

	"github.com/hashicorp/go-hclog"
)

// RaftLogger implements hclog.Logger interface using slog
type RaftLogger struct {
	logger *slog.Logger
	name   string
}

// NewRaftLogger creates a new RaftLogger that wraps a slog.Logger
func NewRaftLogger(logger *slog.Logger) *RaftLogger {
	return &RaftLogger{logger: logger}
}

// Log logs a message at the specified level
func (l *RaftLogger) Log(level hclog.Level, msg string, args ...interface{}) {
	switch level {
	case hclog.Trace:
		l.logger.Debug(msg, args...)
	case hclog.Debug:
		l.logger.Debug(msg, args...)
	case hclog.Info:
		l.logger.Info(msg, args...)
	case hclog.Warn:
		l.logger.Warn(msg, args...)
	case hclog.Error:
		l.logger.Error(msg, args...)
	}
}

// Trace logs a message at trace level
func (l *RaftLogger) Trace(msg string, args ...interface{}) {
	l.Log(hclog.Trace, msg, args...)
}

// Debug logs a message at debug level
func (l *RaftLogger) Debug(msg string, args ...interface{}) {
	l.Log(hclog.Debug, msg, args...)
}

// Info logs a message at info level
func (l *RaftLogger) Info(msg string, args ...interface{}) {
	l.Log(hclog.Info, msg, args...)
}

// Warn logs a message at warn level
func (l *RaftLogger) Warn(msg string, args ...interface{}) {
	l.Log(hclog.Warn, msg, args...)
}

// Error logs a message at error level
func (l *RaftLogger) Error(msg string, args ...interface{}) {
	l.Log(hclog.Error, msg, args...)
}

// IsTrace returns true if trace level logging is enabled
func (l *RaftLogger) IsTrace() bool {
	return l.logger.Enabled(nil, slog.LevelDebug)
}

// IsDebug returns true if debug level logging is enabled
func (l *RaftLogger) IsDebug() bool {
	return l.logger.Enabled(nil, slog.LevelDebug)
}

// IsInfo returns true if info level logging is enabled
func (l *RaftLogger) IsInfo() bool {
	return l.logger.Enabled(nil, slog.LevelInfo)
}

// IsWarn returns true if warn level logging is enabled
func (l *RaftLogger) IsWarn() bool {
	return l.logger.Enabled(nil, slog.LevelWarn)
}

// IsError returns true if error level logging is enabled
func (l *RaftLogger) IsError() bool {
	return l.logger.Enabled(nil, slog.LevelError)
}

// ImpliedArgs returns the implied args
func (l *RaftLogger) ImpliedArgs() []interface{} {
	return nil
}

// With returns a new logger with additional key/value pairs
func (l *RaftLogger) With(args ...interface{}) hclog.Logger {
	return &RaftLogger{logger: l.logger.With(args...), name: l.name}
}

// Named returns a new logger with the specified name
func (l *RaftLogger) Named(name string) hclog.Logger {
	return &RaftLogger{logger: l.logger.With("name", name), name: name}
}

// ResetNamed returns a new logger with the specified name
func (l *RaftLogger) ResetNamed(name string) hclog.Logger {
	return &RaftLogger{logger: l.logger.With("name", name), name: name}
}

// Name returns the logger's name
func (l *RaftLogger) Name() string {
	return l.name
}

// SetLevel sets the logging level
func (l *RaftLogger) SetLevel(level hclog.Level) {
	// Not implemented as slog levels are set at handler creation
}

// GetLevel returns the current logging level
func (l *RaftLogger) GetLevel() hclog.Level {
	// Not implemented as slog levels are set at handler creation
	return hclog.Info
}

// StandardLogger returns a standard logger
func (l *RaftLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(l.StandardWriter(opts), "", 0)
}

// StandardWriter returns a standard writer
func (l *RaftLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return io.Discard
}
