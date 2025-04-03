package logging

import (
	"bytes"
	"io"
	"time"

	"github.com/labstack/echo/v4"
)

// LoggerConfig defines the configuration for the logger middleware
type LoggerConfig struct {
	// Skipper defines a function to skip middleware
	Skipper func(c echo.Context) bool
}

// DefaultLoggerConfig is the default logger middleware config
var DefaultLoggerConfig = LoggerConfig{
	Skipper: func(c echo.Context) bool {
		return false
	},
}

// LoggerMiddleware returns a middleware that logs HTTP requests using slog
func LoggerMiddleware() echo.MiddlewareFunc {
	return LoggerWithConfig(DefaultLoggerConfig)
}

// LoggerWithConfig returns a middleware that logs HTTP requests using slog with custom config
func LoggerWithConfig(config LoggerConfig) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = DefaultLoggerConfig.Skipper
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			req := c.Request()
			res := c.Response()
			start := time.Now()

			// Log request received
			Logger.Info("request received",
				"method", req.Method,
				"uri", req.RequestURI,
				"remote_ip", c.RealIP(),
				"user_agent", req.UserAgent(),
			)

			// Read request body if needed
			var reqBody []byte
			if req.Body != nil {
				reqBody, _ = io.ReadAll(req.Body)
				req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
			}

			// Call next handler
			err := next(c)

			// Calculate latency
			latency := time.Since(start)

			// Prepare log fields
			fields := []interface{}{
				"time", time.Now().Format(time.RFC3339),
				"latency", latency.String(),
				"method", req.Method,
				"uri", req.RequestURI,
				"status", res.Status,
				"remote_ip", c.RealIP(),
				"user_agent", req.UserAgent(),
			}

			// Add request body if present
			if len(reqBody) > 0 {
				fields = append(fields, "request_body", string(reqBody))
			}

			// Add response body if present and error occurred
			if err != nil {
				fields = append(fields, "error", err.Error())
				Logger.Error("request failed", fields...)
			} else {
				Logger.Info("request completed", fields...)
			}

			return err
		}
	}
}
