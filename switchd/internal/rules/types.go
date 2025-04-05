package rules

import (
	"fmt"
	"time"

	"github.com/google/cel-go/cel"
)

// Context represents the evaluation context for a rule
type Context struct {
	Key     string         `json:"key"`
	Context map[string]any `json:"context"`
	Request map[string]any `json:"request"`
	Device  map[string]any `json:"device"`
	Time    time.Time      `json:"time"`
}

// Rule represents a feature flag rule
type Rule struct {
	Expression string `json:"expression"`
	Value      any    `json:"value"`
}

// Program represents a compiled CEL program
type Program struct {
	Expression string
	Compiled   cel.Program
}

// CompilationError represents an error that occurred during expression compilation
type CompilationError struct {
	Expression string
	Issues     []string
}

func (e *CompilationError) Error() string {
	return fmt.Sprintf("compilation failed for expression '%s': %v", e.Expression, e.Issues)
}

// EvaluationError represents an error that occurred during expression evaluation
type EvaluationError struct {
	Expression string
	Context    *Context
	Err        error
}

func (e *EvaluationError) Error() string {
	return fmt.Sprintf("evaluation failed for expression '%s': %v", e.Expression, e.Err)
}
