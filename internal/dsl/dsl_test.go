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
		// 단항 마이너스(-) 미지원: '-' 앞에 피연산자 없이 사용하면 파싱 에러
		{"negative_literal", "-1 > 0"},
		{"leading_minus", "- person"},
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

// TestEvaluate_DoubleNOT: NOT NOT x 는 x 의 truthy와 동일해야 합니다.
func TestEvaluate_DoubleNOT(t *testing.T) {
	counts := map[string]float64{"person": 5, "truck": 0}
	tests := []struct {
		name      string
		expr      string
		wantValue bool
	}{
		// NOT NOT 5 → NOT 0 → 1 → true
		{"not_not_nonzero", "NOT NOT person", true},
		// NOT NOT 0 → NOT 1 → 0 → false
		{"not_not_zero", "NOT NOT truck", false},
		// !! 연산자도 동일하게 동작
		{"bang_bang_nonzero", "!!person", true},
		{"bang_bang_zero", "!!truck", false},
		// NOT NOT 비교식
		{"not_not_comparison", "NOT NOT (person > 3)", true},
		{"not_not_false_comparison", "NOT NOT (person > 10)", false},
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

// TestValidate_EmptyVsNilAllowedIdents: 빈 슬라이스와 nil의 동작 차이를 검증합니다.
// - nil: ident 검사를 건너뜀 → 어떤 ident든 허용
// - 빈 []string{}: ident 허용 목록이 비어있으므로 모든 ident를 거부
func TestValidate_EmptyVsNilAllowedIdents(t *testing.T) {
	t.Run("nil_skips_check", func(t *testing.T) {
		err := Validate("person > 3 AND unknown_obj >= 0", nil)
		if err != nil {
			t.Errorf("nil allowedIdents should skip ident check, got: %v", err)
		}
	})

	t.Run("empty_slice_rejects_all_idents", func(t *testing.T) {
		// 빈 슬라이스는 nil이 아니므로 ident 검사 실행, 허용 목록이 비어 모든 ident 거부
		err := Validate("person > 3", []string{})
		if err == nil {
			t.Error("empty allowedIdents should reject all idents, got nil")
		}
	})

	t.Run("number_only_expr_passes_empty_slice", func(t *testing.T) {
		// ident가 없는 수식은 빈 슬라이스에서도 통과
		err := Validate("42 > 10", []string{})
		if err != nil {
			t.Errorf("number-only expression should pass empty allowedIdents, got: %v", err)
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

// TestCollectIdents_Duplicates: 같은 ident가 여러 번 나오면 중복 포함하여 반환합니다.
// collectIdents는 중복 제거를 하지 않으므로, Validate에서 중복 검사가 발생하지만
// 동일 ident가 allowed 목록에 있으면 정상 통과합니다.
func TestCollectIdents_Duplicates(t *testing.T) {
	tokens, _ := tokenize("person > 3 AND person < 10")
	p := &parser{tokens: tokens}
	node, err := p.parseExpr()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	idents := collectIdents(node)
	count := 0
	for _, id := range idents {
		if id == "person" {
			count++
		}
	}
	// collectIdents는 중복 제거 없이 반환하므로 "person"이 2번 나와야 합니다
	if count != 2 {
		t.Errorf("expected 2 occurrences of 'person' (no dedup), got %d", count)
	}
}

// TestValidate_DuplicateIdentInExpr: 같은 ident가 여러 번 쓰여도 allowed 목록에 있으면 통과합니다.
func TestValidate_DuplicateIdentInExpr(t *testing.T) {
	err := Validate("person > 3 AND person < 10", []string{"person"})
	if err != nil {
		t.Errorf("duplicate ident in allowed list should pass, got: %v", err)
	}
}
