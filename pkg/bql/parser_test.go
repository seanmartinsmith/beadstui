// Tests adapted from github.com/zjrosen/perles (MIT License).
// Rewritten for stdlib testing (no testify).

package bql

import (
	"testing"
)

func TestParse_SimpleComparison(t *testing.T) {
	tests := []struct {
		name  string
		input string
		field string
		op    TokenType
		val   string
	}{
		{"equals string", "type = task", "type", TokenEq, "task"},
		{"not equals", "status != closed", "status", TokenNeq, "closed"},
		{"contains", "title ~ auth", "title", TokenContains, "auth"},
		{"not contains", "title !~ test", "title", TokenNotContains, "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.input, err)
			}
			cmp, ok := query.Filter.(*CompareExpr)
			if !ok {
				t.Fatalf("expected CompareExpr, got %T", query.Filter)
			}
			if cmp.Field != tt.field {
				t.Errorf("field = %q, want %q", cmp.Field, tt.field)
			}
			if cmp.Op != tt.op {
				t.Errorf("op = %v, want %v", cmp.Op, tt.op)
			}
			if cmp.Value.String != tt.val {
				t.Errorf("value = %q, want %q", cmp.Value.String, tt.val)
			}
		})
	}
}

func TestParse_Priority(t *testing.T) {
	query, err := Parse("priority < P2")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cmp := query.Filter.(*CompareExpr)
	if cmp.Value.Type != ValuePriority {
		t.Errorf("value type = %v, want ValuePriority", cmp.Value.Type)
	}
	if cmp.Value.Int != 2 {
		t.Errorf("priority int = %d, want 2", cmp.Value.Int)
	}
}

func TestParse_Boolean(t *testing.T) {
	query, err := Parse("blocked = true")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cmp := query.Filter.(*CompareExpr)
	if cmp.Value.Type != ValueBool {
		t.Errorf("value type = %v, want ValueBool", cmp.Value.Type)
	}
	if !cmp.Value.Bool {
		t.Errorf("value bool = false, want true")
	}
}

func TestParse_InExpression(t *testing.T) {
	query, err := Parse("type in (bug, task)")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	in, ok := query.Filter.(*InExpr)
	if !ok {
		t.Fatalf("expected InExpr, got %T", query.Filter)
	}
	if in.Field != "type" {
		t.Errorf("field = %q, want %q", in.Field, "type")
	}
	if in.Not {
		t.Error("expected Not=false")
	}
	if len(in.Values) != 2 {
		t.Fatalf("values len = %d, want 2", len(in.Values))
	}
	if in.Values[0].String != "bug" || in.Values[1].String != "task" {
		t.Errorf("values = [%q, %q], want [bug, task]", in.Values[0].String, in.Values[1].String)
	}
}

func TestParse_NotInExpression(t *testing.T) {
	query, err := Parse("label not in (backlog, deferred)")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	in := query.Filter.(*InExpr)
	if !in.Not {
		t.Error("expected Not=true")
	}
}

func TestParse_BinaryExpr(t *testing.T) {
	query, err := Parse("type = bug and priority = P0")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	bin, ok := query.Filter.(*BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", query.Filter)
	}
	if bin.Op != TokenAnd {
		t.Errorf("op = %v, want AND", bin.Op)
	}
	left := bin.Left.(*CompareExpr)
	right := bin.Right.(*CompareExpr)
	if left.Field != "type" || right.Field != "priority" {
		t.Errorf("fields = [%q, %q], want [type, priority]", left.Field, right.Field)
	}
}

func TestParse_NotExpr(t *testing.T) {
	query, err := Parse("not blocked = true")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	not, ok := query.Filter.(*NotExpr)
	if !ok {
		t.Fatalf("expected NotExpr, got %T", query.Filter)
	}
	cmp := not.Expr.(*CompareExpr)
	if cmp.Field != "blocked" {
		t.Errorf("field = %q, want blocked", cmp.Field)
	}
}

func TestParse_Parentheses(t *testing.T) {
	query, err := Parse("(type = bug or type = task) and priority < P2")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	bin := query.Filter.(*BinaryExpr)
	if bin.Op != TokenAnd {
		t.Errorf("top op = %v, want AND", bin.Op)
	}
	inner := bin.Left.(*BinaryExpr)
	if inner.Op != TokenOr {
		t.Errorf("inner op = %v, want OR", inner.Op)
	}
}

func TestParse_OrderBy(t *testing.T) {
	query, err := Parse("status = open order by priority asc, created_at desc")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(query.OrderBy) != 2 {
		t.Fatalf("orderby len = %d, want 2", len(query.OrderBy))
	}
	if query.OrderBy[0].Field != "priority" || query.OrderBy[0].Desc {
		t.Errorf("orderby[0] = {%q, desc=%v}, want {priority, false}", query.OrderBy[0].Field, query.OrderBy[0].Desc)
	}
	if query.OrderBy[1].Field != "created_at" || !query.OrderBy[1].Desc {
		t.Errorf("orderby[1] = {%q, desc=%v}, want {created_at, true}", query.OrderBy[1].Field, query.OrderBy[1].Desc)
	}
}

func TestParse_OrderByOnly(t *testing.T) {
	query, err := Parse("order by priority")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if query.Filter != nil {
		t.Error("expected nil filter for order-by-only query")
	}
	if len(query.OrderBy) != 1 {
		t.Fatalf("orderby len = %d, want 1", len(query.OrderBy))
	}
}

func TestParse_DateValues(t *testing.T) {
	tests := []struct {
		input string
		date  string
	}{
		{"created_at > today", "today"},
		{"created_at > yesterday", "yesterday"},
		{"created_at > -7d", "-7d"},
		{"created_at > -24h", "-24h"},
		{"created_at > -3m", "-3m"},
	}

	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			query, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.input, err)
			}
			cmp := query.Filter.(*CompareExpr)
			if cmp.Value.Type != ValueDate {
				t.Errorf("type = %v, want ValueDate", cmp.Value.Type)
			}
			if cmp.Value.String != tt.date {
				t.Errorf("date = %q, want %q", cmp.Value.String, tt.date)
			}
		})
	}
}

func TestParse_QuotedString(t *testing.T) {
	query, err := Parse(`title = "hello world"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cmp := query.Filter.(*CompareExpr)
	if cmp.Value.String != "hello world" {
		t.Errorf("value = %q, want %q", cmp.Value.String, "hello world")
	}
}

func TestParse_Expand(t *testing.T) {
	query, err := Parse("type = epic expand down depth 3")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !query.HasExpand() {
		t.Fatal("expected expand clause")
	}
	if query.Expand.Type != ExpandDown {
		t.Errorf("expand type = %v, want down", query.Expand.Type)
	}
	if query.Expand.Depth != 3 {
		t.Errorf("expand depth = %d, want 3", query.Expand.Depth)
	}
}

func TestParse_ExpandUnlimited(t *testing.T) {
	query, err := Parse("expand all depth *")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if query.Expand.Depth != DepthUnlimited {
		t.Errorf("depth = %d, want DepthUnlimited", query.Expand.Depth)
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing value", "type ="},
		{"missing operator", "type task"},
		{"unclosed paren", "(type = bug"},
		{"invalid depth", "expand down depth 0"},
		{"depth too large", "expand down depth 99"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err == nil {
				t.Errorf("Parse(%q): expected error, got nil", tt.input)
			}
		})
	}
}

func TestParse_ComplexQuery(t *testing.T) {
	input := `(type = bug or type = feature) and priority <= P1 and status != closed order by priority asc`
	query, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if query.Filter == nil {
		t.Fatal("expected non-nil filter")
	}
	if len(query.OrderBy) != 1 {
		t.Fatalf("orderby len = %d, want 1", len(query.OrderBy))
	}
}
