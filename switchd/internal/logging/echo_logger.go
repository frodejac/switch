package logging

import (
	"io"
	"log/slog"

	"github.com/labstack/gommon/log"
)

// EchoLogger implements echo.Logger interface using slog
type EchoLogger struct {
	logger *slog.Logger
}

// NewEchoLogger creates a new EchoLogger that wraps a slog.Logger
func NewEchoLogger(logger *slog.Logger) *EchoLogger {
	return &EchoLogger{logger: logger}
}

// Output returns the logger's output writer
func (l *EchoLogger) Output() io.Writer {
	return io.Discard // Echo doesn't actually use this
}

// SetOutput is a no-op since we use slog's handler
func (l *EchoLogger) SetOutput(w io.Writer) {}

// Prefix returns the logger's prefix
func (l *EchoLogger) Prefix() string {
	return ""
}

// SetPrefix is a no-op since we use slog's handler
func (l *EchoLogger) SetPrefix(p string) {}

// Level returns the logger's level
func (l *EchoLogger) Level() log.Lvl {
	return log.DEBUG // Echo doesn't actually use this
}

// SetLevel is a no-op since we use slog's handler
func (l *EchoLogger) SetLevel(v log.Lvl) {}

// SetHeader is a no-op since we use slog's handler
func (l *EchoLogger) SetHeader(h string) {}

// Print logs a message at info level
func (l *EchoLogger) Print(i ...any) {
	l.logger.Info("", "message", i)
}

// Printf logs a formatted message at info level
func (l *EchoLogger) Printf(format string, args ...any) {
	l.logger.Info(format, "args", args)
}

// Printj logs a JSON message at info level
func (l *EchoLogger) Printj(j log.JSON) {
	l.logger.Info("", "json", j)
}

// Debug logs a message at debug level
func (l *EchoLogger) Debug(i ...any) {
	l.logger.Debug("", "message", i)
}

// Debugf logs a formatted message at debug level
func (l *EchoLogger) Debugf(format string, args ...any) {
	l.logger.Debug(format, "args", args)
}

// Debugj logs a JSON message at debug level
func (l *EchoLogger) Debugj(j log.JSON) {
	l.logger.Debug("", "json", j)
}

// Info logs a message at info level
func (l *EchoLogger) Info(i ...any) {
	l.logger.Info("", "message", i)
}

// Infof logs a formatted message at info level
func (l *EchoLogger) Infof(format string, args ...any) {
	l.logger.Info(format, "args", args)
}

// Infoj logs a JSON message at info level
func (l *EchoLogger) Infoj(j log.JSON) {
	l.logger.Info("", "json", j)
}

// Warn logs a message at warn level
func (l *EchoLogger) Warn(i ...any) {
	l.logger.Warn("", "message", i)
}

// Warnf logs a formatted message at warn level
func (l *EchoLogger) Warnf(format string, args ...any) {
	l.logger.Warn(format, "args", args)
}

// Warnj logs a JSON message at warn level
func (l *EchoLogger) Warnj(j log.JSON) {
	l.logger.Warn("", "json", j)
}

// Error logs a message at error level
func (l *EchoLogger) Error(i ...any) {
	l.logger.Error("", "message", i)
}

// Errorf logs a formatted message at error level
func (l *EchoLogger) Errorf(format string, args ...any) {
	l.logger.Error(format, "args", args)
}

// Errorj logs a JSON message at error level
func (l *EchoLogger) Errorj(j log.JSON) {
	l.logger.Error("", "json", j)
}

// Fatal logs a message at error level and exits
func (l *EchoLogger) Fatal(i ...any) {
	l.logger.Error("", "message", i)
}

// Fatalf logs a formatted message at error level and exits
func (l *EchoLogger) Fatalf(format string, args ...any) {
	l.logger.Error(format, "args", args)
}

// Fatalj logs a JSON message at error level and exits
func (l *EchoLogger) Fatalj(j log.JSON) {
	l.logger.Error("", "json", j)
}

// Panic logs a message at error level and panics
func (l *EchoLogger) Panic(i ...any) {
	l.logger.Error("", "message", i)
}

// Panicf logs a formatted message at error level and panics
func (l *EchoLogger) Panicf(format string, args ...any) {
	l.logger.Error(format, "args", args)
}

// Panicj logs a JSON message at error level and panics
func (l *EchoLogger) Panicj(j log.JSON) {
	l.logger.Error("", "json", j)
}
