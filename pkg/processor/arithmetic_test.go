package processor

import (
	"errors"
	"testing"
)

func TestEvalArithmeticDivideByZero(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{name: "division", expr: "result=10/0"},
		{name: "modulo", expr: "result=10%0"},
		{name: "compound division", expr: "result/=0"},
		{name: "compound modulo", expr: "result%=0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewEnvironment(true)
			env.Set("result", "7")
			p := New(env, nil, nil)

			if _, err := p.EvalArithmetic(tt.expr); !errors.Is(err, ErrDivideByZero) {
				t.Fatalf("EvalArithmetic(%q) error = %v, want %v", tt.expr, err, ErrDivideByZero)
			}
			if got, _ := env.Get("result"); got != "7" {
				t.Errorf("result = %q, want %q", got, "7")
			}
		})
	}
}
