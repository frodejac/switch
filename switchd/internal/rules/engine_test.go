package rules

import (
	"testing"
	"time"
)

func TestEngine_Compile(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	tests := []struct {
		name       string
		expression string
		wantErr    bool
	}{
		{
			name:       "valid expression",
			expression: "request.user_id == 'abc123'",
			wantErr:    false,
		},
		{
			name:       "invalid expression",
			expression: "invalid syntax ===",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := engine.Compile(tt.expression)
			if (err != nil) != tt.wantErr {
				t.Errorf("Compile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && program.Expression != tt.expression {
				t.Errorf("Program expression = %v, want %v", program.Expression, tt.expression)
			}
		})
	}
}

func TestEngine_Evaluate(t *testing.T) {
	engine, _ := NewEngine()

	tests := []struct {
		name       string
		expression string
		context    *Context
		want       bool
		wantErr    bool
	}{
		{
			name:       "simple comparison",
			expression: "request.user_id == 'abc123'",
			context: &Context{
				Key: "test",
				Request: map[string]any{
					"user_id": "abc123",
				},
				Time: time.Now(),
			},
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := engine.Compile(tt.expression)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}

			got, err := engine.Evaluate(program, tt.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}
