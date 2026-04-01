// Tests adapted from github.com/zjrosen/perles (MIT License).
// Rewritten for stdlib testing (no testify).

package bql

import "testing"

func TestLexer_BasicTokens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Token
	}{
		{
			name:  "simple equality",
			input: "type = task",
			expected: []Token{
				{Type: TokenIdent, Literal: "type"},
				{Type: TokenEq, Literal: "="},
				{Type: TokenIdent, Literal: "task"},
				{Type: TokenEOF},
			},
		},
		{
			name:  "priority comparison",
			input: "priority < P2",
			expected: []Token{
				{Type: TokenIdent, Literal: "priority"},
				{Type: TokenLt, Literal: "<"},
				{Type: TokenIdent, Literal: "P2"},
				{Type: TokenEOF},
			},
		},
		{
			name:  "and/or keywords",
			input: "type = bug and priority = P0",
			expected: []Token{
				{Type: TokenIdent, Literal: "type"},
				{Type: TokenEq, Literal: "="},
				{Type: TokenIdent, Literal: "bug"},
				{Type: TokenAnd, Literal: "and"},
				{Type: TokenIdent, Literal: "priority"},
				{Type: TokenEq, Literal: "="},
				{Type: TokenIdent, Literal: "P0"},
				{Type: TokenEOF},
			},
		},
		{
			name:  "contains and not contains",
			input: "title ~ auth",
			expected: []Token{
				{Type: TokenIdent, Literal: "title"},
				{Type: TokenContains, Literal: "~"},
				{Type: TokenIdent, Literal: "auth"},
				{Type: TokenEOF},
			},
		},
		{
			name:  "date offset",
			input: "created_at > -7d",
			expected: []Token{
				{Type: TokenIdent, Literal: "created_at"},
				{Type: TokenGt, Literal: ">"},
				{Type: TokenNumber, Literal: "-7d"},
				{Type: TokenEOF},
			},
		},
		{
			name:  "quoted string",
			input: `title = "hello world"`,
			expected: []Token{
				{Type: TokenIdent, Literal: "title"},
				{Type: TokenEq, Literal: "="},
				{Type: TokenString, Literal: "hello world"},
				{Type: TokenEOF},
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: []Token{{Type: TokenEOF}},
		},
		{
			name:  "identifier with hyphens",
			input: "id = bt-123",
			expected: []Token{
				{Type: TokenIdent, Literal: "id"},
				{Type: TokenEq, Literal: "="},
				{Type: TokenIdent, Literal: "bt-123"},
				{Type: TokenEOF},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			for i, expected := range tt.expected {
				tok := lexer.NextToken()
				if tok.Type != expected.Type {
					t.Errorf("token %d type = %v, want %v", i, tok.Type, expected.Type)
				}
				if expected.Literal != "" && tok.Literal != expected.Literal {
					t.Errorf("token %d literal = %q, want %q", i, tok.Literal, expected.Literal)
				}
			}
		})
	}
}

func TestLexer_AllOperators(t *testing.T) {
	operators := map[string]TokenType{
		"=":  TokenEq,
		"!=": TokenNeq,
		"<":  TokenLt,
		">":  TokenGt,
		"<=": TokenLte,
		">=": TokenGte,
		"~":  TokenContains,
		"!~": TokenNotContains,
	}

	for op, expected := range operators {
		t.Run(op, func(t *testing.T) {
			lexer := NewLexer("field " + op + " value")
			lexer.NextToken() // skip field
			tok := lexer.NextToken()
			if tok.Type != expected {
				t.Errorf("type = %v, want %v", tok.Type, expected)
			}
		})
	}
}

func TestLexer_CaseInsensitiveKeywords(t *testing.T) {
	for _, kw := range []string{"and", "AND", "And", "or", "OR", "not", "NOT", "in", "IN", "true", "TRUE", "false", "FALSE"} {
		t.Run(kw, func(t *testing.T) {
			lexer := NewLexer(kw)
			tok := lexer.NextToken()
			if tok.Type == TokenIdent {
				t.Errorf("%q parsed as ident, expected keyword", kw)
			}
		})
	}
}
