package dsl

import (
	"math"
	"testing"
)

func TestEvaluate_Arithmetic(t *testing.T) {
	counts := map[string]float64{
		"person": 5,
		"car":    3,
		"truck":  0,
	}

	tests := []struct {
		name       string
		expr       string
		wantValue  bool
		wantRaw    float64
		wantErrStr string // EvalResult.Error, not Go error
	}{
		// basic ident
		{"ident_person", "person", true, 5, ""},
		{"ident_truck_zero", "truck", false, 0, ""},
		{"ident_missing", "unknown", false, 0, ""},

		// addition / subtraction
		{"add", "person + car", true, 8, ""},
		{"sub", "person - car", true, 2, ""},
		{"sub_to_zero", "car - car", false, 0, ""},

		// multiplication
		{"mul", "person * 2", true, 10, ""},
		{"mul_zero", "person * 0", false, 0, ""},

		// division (integer division)
		{"div", "person / car", true, 1, ""},     // int(5/3) = 1
		{"div_exact", "10 / 5", true, 2, ""},     // 10/5 = 2
		{"div_by_zero", "person / 0", false, 0, "DIVIDE_BY_ZERO"},
		{"div_by_zero_ident", "person / truck", false, 0, "DIVIDE_BY_ZERO"},

		// number literal
		{"number", "42", true, 42, ""},
		{"number_zero", "0", false, 0, ""},
		{"number_decimal", "3.14", true, 3.14, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.expr, counts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Value != tt.wantValue {
				t.Errorf("Value = %v, want %v", result.Value, tt.wantValue)
			}
			if math.Abs(result.Raw-tt.wantRaw) > 1e-9 {
				t.Errorf("Raw = %v, want %v", result.Raw, tt.wantRaw)
			}
			if result.Error != tt.wantErrStr {
				t.Errorf("Error = %q, want %q", result.Error, tt.wantErrStr)
			}
		})
	}
}

func TestEvaluate_Comparison(t *testing.T) {
	counts := map[string]float64{
		"person": 5,
		"car":    3,
	}

	tests := []struct {
		name      string
		expr      string
		wantValue bool
		wantRaw   float64
	}{
		{"gt_true", "person > 3", true, 1},
		{"gt_false", "person > 5", false, 0},
		{"lt_true", "car < 5", true, 1},
		{"lt_false", "car < 3", false, 0},
		{"gte_true_eq", "person >= 5", true, 1},
		{"gte_true_gt", "person >= 3", true, 1},
		{"gte_false", "car >= 5", false, 0},
		{"lte_true_eq", "car <= 3", true, 1},
		{"lte_true_lt", "car <= 5", true, 1},
		{"lte_false", "person <= 3", false, 0},
		{"eq_true", "person == 5", true, 1},
		{"eq_false", "person == 3", false, 0},
		{"neq_true", "person != 3", true, 1},
		{"neq_false", "person != 5", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.expr, counts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Value != tt.wantValue {
				t.Errorf("Value = %v, want %v", result.Value, tt.wantValue)
			}
			if result.Raw != tt.wantRaw {
				t.Errorf("Raw = %v, want %v", result.Raw, tt.wantRaw)
			}
		})
	}
}

func TestEvaluate_Logical(t *testing.T) {
	counts := map[string]float64{
		"person": 5,
		"car":    3,
		"truck":  0,
	}

	tests := []struct {
		name      string
		expr      string
		wantValue bool
	}{
		{"and_true", "person > 3 AND car > 1", true},
		{"and_false_left", "truck > 0 AND car > 1", false},
		{"and_false_right", "person > 3 AND truck > 0", false},
		{"and_both_false", "truck > 0 AND truck > 1", false},

		{"or_true_both", "person > 3 OR car > 1", true},
		{"or_true_left", "person > 3 OR truck > 0", true},
		{"or_true_right", "truck > 0 OR car > 1", true},
		{"or_false", "truck > 0 OR truck > 1", false},

		{"not_true", "NOT truck", true},       // NOT 0 → 1
		{"not_false", "NOT person", false},     // NOT 5 → 0
		{"not_bang", "!truck", true},           // ! 0 → 1
		{"not_bang_expr", "!(person > 10)", true},

		// complex
		{"complex_and_or", "person > 3 AND car > 1 OR truck > 0", true},
		{"complex_or_and", "truck > 0 OR person > 3 AND car > 1", true},
		{"not_and", "NOT truck AND person > 3", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.expr, counts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Value != tt.wantValue {
				t.Errorf("Value = %v, want %v", result.Value, tt.wantValue)
			}
		})
	}
}

func TestEvaluate_Parentheses(t *testing.T) {
	counts := map[string]float64{
		"person": 5,
		"car":    3,
		"truck":  0,
	}

	tests := []struct {
		name      string
		expr      string
		wantValue bool
		wantRaw   float64
	}{
		{"paren_add", "(person + car) > 7", true, 1},
		{"paren_add_false", "(person + car) > 10", false, 0},
		{"paren_group", "(person > 3) AND (car > 1)", true, 1},
		{"paren_override_precedence", "(truck > 0 OR person > 3) AND car > 1", true, 1},
		{"nested_paren", "((person + car) * 2) > 15", true, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.expr, counts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Value != tt.wantValue {
				t.Errorf("Value = %v, want %v", result.Value, tt.wantValue)
			}
			if result.Raw != tt.wantRaw {
				t.Errorf("Raw = %v, want %v", result.Raw, tt.wantRaw)
			}
		})
	}
}

func TestEvaluate_Precedence(t *testing.T) {
	counts := map[string]float64{
		"a": 2,
		"b": 3,
		"c": 4,
	}

	tests := []struct {
		name    string
		expr    string
		wantRaw float64
	}{
		// * before +
		{"mul_before_add", "a + b * c", 14},      // 2 + (3*4) = 14
		{"paren_override", "(a + b) * c", 20},     // (2+3) * 4 = 20

		// comparison returns 0 or 1
		{"cmp_result", "a > 1", 1},
		{"cmp_in_and", "a > 1 AND b > 2", 1},     // (2>1) AND (3>2) → 1 AND 1 → 1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.expr, counts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(result.Raw-tt.wantRaw) > 1e-9 {
				t.Errorf("Raw = %v, want %v", result.Raw, tt.wantRaw)
			}
		})
	}
}

func TestEvaluate_EdgeCases(t *testing.T) {
	t.Run("empty_counts", func(t *testing.T) {
		result, err := Evaluate("person > 0", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != false {
			t.Error("expected false for nil counts")
		}
	})

	t.Run("missing_ident_defaults_zero", func(t *testing.T) {
		result, err := Evaluate("missing_obj == 0", map[string]float64{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Value {
			t.Error("expected true: missing ident should be 0")
		}
	})

	t.Run("divide_by_zero_in_complex", func(t *testing.T) {
		counts := map[string]float64{"a": 10, "b": 0}
		result, err := Evaluate("a / b > 1", counts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Error != "DIVIDE_BY_ZERO" {
			t.Errorf("Error = %q, want DIVIDE_BY_ZERO", result.Error)
		}
		if result.Value != false {
			t.Error("expected false on divide by zero")
		}
	})

	t.Run("whitespace_tolerance", func(t *testing.T) {
		counts := map[string]float64{"x": 5}
		result, err := Evaluate("  x  +  2  >  6  ", counts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Value {
			t.Error("expected true: 5+2 > 6")
		}
	})

	t.Run("underscore_ident", func(t *testing.T) {
		counts := map[string]float64{"my_object": 3}
		result, err := Evaluate("my_object > 2", counts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Value {
			t.Error("expected true")
		}
	})
}

func TestEvaluate_ParseErrors(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"empty", ""},
		{"unclosed_paren", "(person > 3"},
		{"double_op", "person >> 3"},
		{"trailing_op", "person +"},
		{"invalid_char", "person @ 3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Evaluate(tt.expr, map[string]float64{"person": 5})
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Run("valid_expression", func(t *testing.T) {
		err := Validate("person > 3 AND car >= 1", []string{"person", "car"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("unknown_ident", func(t *testing.T) {
		err := Validate("person > 3 AND truck >= 1", []string{"person", "car"})
		if err == nil {
			t.Error("expected error for unknown ident 'truck'")
		}
	})

	t.Run("nil_allowed_skips_check", func(t *testing.T) {
		err := Validate("anything > 3", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("syntax_error", func(t *testing.T) {
		err := Validate("person >", []string{"person"})
		if err == nil {
			t.Error("expected syntax error")
		}
	})

	t.Run("complex_valid", func(t *testing.T) {
		err := Validate("(person + car) * 2 > 10 AND NOT truck == 0", []string{"person", "car", "truck"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestCollectIdents(t *testing.T) {
	tokens, _ := tokenize("person > 3 AND car >= 1 OR truck == 0")
	p := &parser{tokens: tokens}
	node, err := p.parseExpr()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	idents := collectIdents(node)
	want := map[string]bool{"person": true, "car": true, "truck": true}
	got := make(map[string]bool)
	for _, id := range idents {
		got[id] = true
	}

	for k := range want {
		if !got[k] {
			t.Errorf("missing ident %q", k)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("unexpected ident %q", k)
		}
	}
}
