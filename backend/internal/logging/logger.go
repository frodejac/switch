package logging

import (
	"log/slog"
	"os"
)

var (
	// Logger is the global logger instance
	Logger *slog.Logger
)

// Init initializes the global logger with the specified level
func Init(level slog.Level) {
	Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	Logger.Debug(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	Logger.Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	Logger.Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	Logger.Error(msg, args...)
}

// With returns a logger with the specified attributes
func With(args ...any) *slog.Logger {
	return Logger.With(args...)
}
