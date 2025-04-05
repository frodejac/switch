package rules

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Engine handles rule compilation and evaluation
type Engine struct {
	env *cel.Env
}

// NewEngine creates a new rules engine
func NewEngine() (*Engine, error) {
	env, err := cel.NewEnv(
		cel.Variable("key", cel.StringType),
		cel.Variable("context", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("request", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("device", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("time", cel.TimestampType),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return &Engine{
		env: env,
	}, nil
}

// Compile compiles a rule expression
func (e *Engine) Compile(expression string) (*Program, error) {
	ast, issues := e.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		// Convert issues to a more readable format
		var issueStrings []string
		for _, issue := range issues.Errors() {
			issueStrings = append(issueStrings, issue.Message)
		}
		return nil, &CompilationError{
			Expression: expression,
			Issues:     issueStrings,
		}
	}

	program, err := e.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("program creation failed: %w", err)
	}

	return &Program{
		Expression: expression,
		Compiled:   program,
	}, nil
}

func (e *Engine) Evaluate(program *Program, ctx *Context) (any, error) {
	// Convert context to CEL-compatible types
	vars, err := e.prepareVars(ctx)
	if err != nil {
		return false, &EvaluationError{
			Expression: program.Expression,
			Context:    ctx,
			Err:        fmt.Errorf("failed to prepare variables: %w", err),
		}
	}

	// Evaluate the program
	result, _, err := program.Compiled.Eval(vars)
	if err != nil {
		return false, &EvaluationError{
			Expression: program.Expression,
			Context:    ctx,
			Err:        fmt.Errorf("evaluation failed: %w", err),
		}
	}

	// Return the raw value
	return result.Value(), nil
}

// prepareVars converts context to CEL-compatible types
func (e *Engine) prepareVars(ctx *Context) (map[string]any, error) {
	// Convert context to a regular map
	contextMap := make(map[string]any)
	for k, v := range ctx.Context {
		contextMap[k] = v
	}

	// Convert request to a regular map
	requestMap := make(map[string]any)
	for k, v := range ctx.Request {
		// Handle nested maps
		if nestedMap, ok := v.(map[string]any); ok {
			requestMap[k] = nestedMap
		} else if nestedMap, ok := v.(map[string]string); ok {
			// Convert map[string]string to map[string]any
			interfaceMap := make(map[string]any)
			for k, v := range nestedMap {
				interfaceMap[k] = v
			}
			requestMap[k] = interfaceMap
		} else {
			requestMap[k] = v
		}
	}

	// Convert device to a regular map
	deviceMap := make(map[string]any)
	for k, v := range ctx.Device {
		deviceMap[k] = v
	}

	// Convert maps to protobuf structs
	contextStruct, err := structpb.NewStruct(contextMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert context: %w", err)
	}

	requestStruct, err := structpb.NewStruct(requestMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	deviceStruct, err := structpb.NewStruct(deviceMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert device: %w", err)
	}

	return map[string]any{
		"key":     ctx.Key,
		"context": contextStruct,
		"request": requestStruct,
		"device":  deviceStruct,
		"time":    timestamppb.New(ctx.Time),
	}, nil
}
