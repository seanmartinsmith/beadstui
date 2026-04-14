// Adapted from github.com/zjrosen/perles (MIT License). See LICENSE in this directory.
// Stripped SQLite dialect (bt is Dolt-only). Replaced appbeads.SQLDialect with local type.
//
// Used by DoltExecutor (not yet implemented). See brainstorm for design:
// docs/brainstorms/2026-04-01-bql-import-brainstorm.md

package bql

import (
	"fmt"
	"strings"
)

// SQLBuilder converts a BQL AST to SQL for Dolt (MySQL dialect).
type SQLBuilder struct {
	query      *Query
	params     []any
	blockedSQL func(isBlocked bool) string
}

// SQLBuilderOption configures a SQLBuilder.
type SQLBuilderOption func(*SQLBuilder)

// WithBlockedSQL overrides the default blocked field SQL generation.
func WithBlockedSQL(fn func(isBlocked bool) string) SQLBuilderOption {
	return func(b *SQLBuilder) { b.blockedSQL = fn }
}

// NewSQLBuilder creates a builder for the query.
func NewSQLBuilder(query *Query, opts ...SQLBuilderOption) *SQLBuilder {
	b := &SQLBuilder{query: query}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Build generates the SQL WHERE clause and ORDER BY.
func (b *SQLBuilder) Build() (whereClause string, orderBy string, params []any) {
	if b.query.Filter != nil {
		whereClause = b.buildExpr(b.query.Filter)
	}

	if len(b.query.OrderBy) > 0 {
		orderBy = b.buildOrderBy()
	}

	return whereClause, orderBy, b.params
}

// buildExpr recursively builds SQL for an expression.
func (b *SQLBuilder) buildExpr(expr Expr) string {
	switch e := expr.(type) {
	case *BinaryExpr:
		left := b.buildExpr(e.Left)
		right := b.buildExpr(e.Right)
		op := "AND"
		if e.Op == TokenOr {
			op = "OR"
		}
		return fmt.Sprintf("(%s %s %s)", left, op, right)

	case *NotExpr:
		return fmt.Sprintf("NOT (%s)", b.buildExpr(e.Expr))

	case *CompareExpr:
		return b.buildCompare(e)

	case *InExpr:
		return b.buildIn(e)
	}

	return ""
}

// buildCompare builds SQL for a comparison expression.
func (b *SQLBuilder) buildCompare(e *CompareExpr) string {
	switch e.Field {
	case "blocked":
		return b.buildBlockedSQL(e.Value.Bool)

	case "label":
		switch e.Op {
		case TokenContains:
			b.params = append(b.params, "%"+e.Value.String+"%")
			return "i.id IN (SELECT issue_id FROM labels WHERE label LIKE ?)"
		case TokenNotContains:
			b.params = append(b.params, "%"+e.Value.String+"%")
			return "i.id NOT IN (SELECT issue_id FROM labels WHERE label LIKE ?)"
		case TokenNeq:
			b.params = append(b.params, e.Value.String)
			return "i.id NOT IN (SELECT issue_id FROM labels WHERE label = ?)"
		default: // TokenEq
			b.params = append(b.params, e.Value.String)
			return "i.id IN (SELECT issue_id FROM labels WHERE label = ?)"
		}
	}

	column := b.fieldToColumn(e.Field)

	if e.Field == "priority" {
		b.params = append(b.params, e.Value.Int)
		return fmt.Sprintf("%s %s ?", column, b.opToSQL(e.Op))
	}

	if e.Value.Type == ValueDate {
		dateSQL := b.dateToSQL(e.Value.String)
		return fmt.Sprintf("%s %s %s", column, b.opToSQL(e.Op), dateSQL)
	}

	if e.Field == "assignee" {
		switch e.Op {
		case TokenContains:
			b.params = append(b.params, "%"+e.Value.String+"%")
			return fmt.Sprintf("COALESCE(%s, '') LIKE ?", column)
		case TokenNotContains:
			b.params = append(b.params, "%"+e.Value.String+"%")
			return fmt.Sprintf("COALESCE(%s, '') NOT LIKE ?", column)
		default:
			b.params = append(b.params, e.Value.String)
			return fmt.Sprintf("COALESCE(%s, '') %s ?", column, b.opToSQL(e.Op))
		}
	}

	switch e.Op {
	case TokenContains:
		b.params = append(b.params, "%"+e.Value.String+"%")
		return fmt.Sprintf("%s LIKE ?", column)
	case TokenNotContains:
		b.params = append(b.params, "%"+e.Value.String+"%")
		return fmt.Sprintf("%s NOT LIKE ?", column)
	}

	b.params = append(b.params, e.Value.String)
	return fmt.Sprintf("%s %s ?", column, b.opToSQL(e.Op))
}

// buildIn builds SQL for an IN expression.
func (b *SQLBuilder) buildIn(e *InExpr) string {
	if e.Field == "label" {
		placeholders := make([]string, len(e.Values))
		for i, v := range e.Values {
			placeholders[i] = "?"
			b.params = append(b.params, v.String)
		}
		subquery := fmt.Sprintf("i.id IN (SELECT issue_id FROM labels WHERE label IN (%s))",
			strings.Join(placeholders, ", "))
		if e.Not {
			return "NOT " + subquery
		}
		return subquery
	}

	column := b.fieldToColumn(e.Field)
	placeholders := make([]string, len(e.Values))

	for i, v := range e.Values {
		placeholders[i] = "?"
		if e.Field == "priority" {
			b.params = append(b.params, v.Int)
		} else {
			b.params = append(b.params, v.String)
		}
	}

	op := "IN"
	if e.Not {
		op = "NOT IN"
	}

	return fmt.Sprintf("%s %s (%s)", column, op, strings.Join(placeholders, ", "))
}

// fieldToColumn maps BQL field names to SQL column names.
func (b *SQLBuilder) fieldToColumn(field string) string {
	mapping := map[string]string{
		"type":       "i.issue_type",
		"created_at": "i.created_at",
		"updated_at": "i.updated_at",
		"due_date":   "i.due_at",
		"closed_at":  "i.closed_at",
	}
	if col, ok := mapping[field]; ok {
		return col
	}
	return "i." + field
}

// opToSQL converts a token operator to SQL.
func (b *SQLBuilder) opToSQL(op TokenType) string {
	switch op {
	case TokenEq:
		return "="
	case TokenNeq:
		return "!="
	case TokenLt:
		return "<"
	case TokenGt:
		return ">"
	case TokenLte:
		return "<="
	case TokenGte:
		return ">="
	default:
		return "="
	}
}

// dateToSQL converts a date value to a MySQL/Dolt SQL expression.
func (b *SQLBuilder) dateToSQL(dateStr string) string {
	switch dateStr {
	case "today":
		return "CURDATE()"
	case "yesterday":
		return "DATE_SUB(CURDATE(), INTERVAL 1 DAY)"
	default:
		if len(dateStr) > 1 && dateStr[0] == '-' {
			suffix := dateStr[len(dateStr)-1]
			value := dateStr[1 : len(dateStr)-1]

			switch suffix {
			case 'd', 'D':
				b.params = append(b.params, value)
				return "DATE_SUB(CURDATE(), INTERVAL ? DAY)"
			case 'h', 'H':
				b.params = append(b.params, value)
				return "DATE_SUB(NOW(), INTERVAL ? HOUR)"
			case 'm', 'M':
				b.params = append(b.params, value)
				return "DATE_SUB(CURDATE(), INTERVAL ? MONTH)"
			}
		}
		b.params = append(b.params, dateStr)
		return "?"
	}
}

// doltBlockedSubquery finds blocked issues via inline SQL (bypasses views).
const doltBlockedSubquery = `SELECT bi.id FROM issues bi
WHERE bi.status IN ('open', 'in_progress', 'blocked', 'deferred', 'hooked')
AND EXISTS (
  SELECT 1 FROM dependencies d
  WHERE d.issue_id = bi.id AND d.type = 'blocks'
  AND EXISTS (
    SELECT 1 FROM issues blocker
    WHERE blocker.id = d.depends_on_id
    AND blocker.status IN ('open', 'in_progress', 'blocked', 'deferred', 'hooked')
  )
)`

// buildBlockedSQL returns the SQL fragment for the blocked field.
func (b *SQLBuilder) buildBlockedSQL(isBlocked bool) string {
	if b.blockedSQL != nil {
		return b.blockedSQL(isBlocked)
	}
	if isBlocked {
		return "i.id IN (" + doltBlockedSubquery + ")"
	}
	return "i.id NOT IN (" + doltBlockedSubquery + ")"
}

// buildOrderBy builds the ORDER BY clause.
func (b *SQLBuilder) buildOrderBy() string {
	var parts []string
	for _, term := range b.query.OrderBy {
		col := b.fieldToColumn(term.Field)
		dir := "ASC"
		if term.Desc {
			dir = "DESC"
		}
		parts = append(parts, fmt.Sprintf("%s %s", col, dir))
	}
	return strings.Join(parts, ", ")
}
