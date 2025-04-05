package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// BadgerLogger is a wrapper around slog.Logger that implements badger.Logger
type BadgerLogger struct {
	logger *slog.Logger
}

// NewBadgerLogger creates a new BadgerLogger that wraps the given slog.Logger
func NewBadgerLogger(logger *slog.Logger) *BadgerLogger {
	return &BadgerLogger{logger: logger}
}

// Errorf implements badger.Logger
func (l *BadgerLogger) Errorf(format string, args ...any) {
	l.logger.Error(fmt.Sprintf(format, args...))
}

// Warningf implements badger.Logger
func (l *BadgerLogger) Warningf(format string, args ...any) {
	l.logger.Warn(fmt.Sprintf(format, args...))
}

// Infof implements badger.Logger
func (l *BadgerLogger) Infof(format string, args ...any) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

// Debugf implements badger.Logger
func (l *BadgerLogger) Debugf(format string, args ...any) {
	l.logger.Debug(fmt.Sprintf(format, args...))
}

// Output implements badger.Logger
func (l *BadgerLogger) Output(calldepth int, s string) error {
	// Badger uses this for more detailed logging
	// We'll parse the log level from the message and use the appropriate slog level
	switch {
	case strings.Contains(s, "[ERROR]"):
		l.logger.Error(s)
	case strings.Contains(s, "[WARNING]"):
		l.logger.Warn(s)
	case strings.Contains(s, "[INFO]"):
		l.logger.Info(s)
	case strings.Contains(s, "[DEBUG]"):
		l.logger.Debug(s)
	default:
		l.logger.Info(s)
	}
	return nil
}

// SetOutput implements badger.Logger
func (l *BadgerLogger) SetOutput(w io.Writer) {
	// Not needed as we're using slog
}
