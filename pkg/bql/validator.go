// Adapted from github.com/zjrosen/perles (MIT License). See LICENSE in this directory.
// Field list updated for beadstui's model.Issue.

package bql

import (
	"fmt"
	"strings"
)

// ValidFields defines the set of valid field names for bt's issue model.
var ValidFields = map[string]FieldType{
	"id":          FieldString,
	"title":       FieldString,
	"description": FieldString,
	"design":      FieldString,
	"notes":       FieldString,
	"status":      FieldEnum,
	"priority":    FieldPriority,
	"type":        FieldEnum,
	"assignee":    FieldString,
	"label":       FieldString,
	"source_repo": FieldString,
	"blocked":     FieldBool,
	"created_at":  FieldDate,
	"updated_at":  FieldDate,
	"due_date":    FieldDate,
	"closed_at":   FieldDate,
}

// FieldType categorizes fields for validation.
type FieldType int

const (
	FieldString FieldType = iota
	FieldEnum
	FieldPriority
	FieldBool
	FieldDate
)

// ValidTypeValues are the valid values for the type field.
var ValidTypeValues = map[string]bool{
	"bug":     true,
	"feature": true,
	"task":    true,
	"epic":    true,
	"chore":   true,
}

// ValidStatusValues are the valid values for the status field.
var ValidStatusValues = map[string]bool{
	"open":        true,
	"in_progress": true,
	"blocked":     true,
	"deferred":    true,
	"pinned":      true,
	"hooked":      true,
	"review":      true,
	"closed":      true,
	"tombstone":   true,
}

// ValidPriorityValues are the valid values for the priority field.
var ValidPriorityValues = map[string]bool{
	"P0": true, "p0": true,
	"P1": true, "p1": true,
	"P2": true, "p2": true,
	"P3": true, "p3": true,
	"P4": true, "p4": true,
}

// Validate validates a BQL query against the default ValidFields.
func Validate(query *Query) error {
	return ValidateWithFields(query, ValidFields)
}

// ValidateWithFields validates a BQL query against a custom set of valid fields.
func ValidateWithFields(query *Query, validFields map[string]FieldType) error {
	if query.Filter != nil {
		if err := validateExpr(query.Filter, validFields); err != nil {
			return err
		}
	}

	for _, term := range query.OrderBy {
		if err := validateOrderField(term.Field, validFields); err != nil {
			return err
		}
	}

	return nil
}

// validateExpr validates an expression recursively.
func validateExpr(expr Expr, validFields map[string]FieldType) error {
	switch e := expr.(type) {
	case *BinaryExpr:
		if err := validateExpr(e.Left, validFields); err != nil {
			return err
		}
		return validateExpr(e.Right, validFields)

	case *NotExpr:
		return validateExpr(e.Expr, validFields)

	case *CompareExpr:
		return validateCompare(e, validFields)

	case *InExpr:
		return validateIn(e, validFields)
	}

	return nil
}

// validateCompare validates a comparison expression.
func validateCompare(e *CompareExpr, validFields map[string]FieldType) error {
	fieldType, ok := validFields[e.Field]
	if !ok {
		return fmt.Errorf("unknown field: %q (valid: %s)", e.Field, fieldNamesFromMap(validFields))
	}

	if err := validateOperator(e.Field, fieldType, e.Op); err != nil {
		return err
	}

	return validateValue(e.Field, fieldType, e.Value)
}

// validateIn validates an IN expression.
func validateIn(e *InExpr, validFields map[string]FieldType) error {
	fieldType, ok := validFields[e.Field]
	if !ok {
		return fmt.Errorf("unknown field: %q (valid: %s)", e.Field, fieldNamesFromMap(validFields))
	}

	// IN is only valid for enum, string, and priority fields
	if fieldType == FieldBool || fieldType == FieldDate {
		return fmt.Errorf("operator IN is not valid for field %q", e.Field)
	}

	for _, v := range e.Values {
		if err := validateValue(e.Field, fieldType, v); err != nil {
			return err
		}
	}

	return nil
}

// validateOperator checks if an operator is valid for a field type.
func validateOperator(field string, fieldType FieldType, op TokenType) error {
	switch fieldType {
	case FieldBool:
		if op != TokenEq && op != TokenNeq {
			return fmt.Errorf("operator %q is not valid for boolean field %q (use = or !=)", op, field)
		}

	case FieldEnum:
		if op != TokenEq && op != TokenNeq {
			return fmt.Errorf("operator %q is not valid for field %q (use = or !=)", op, field)
		}

	case FieldString:
		if op != TokenEq && op != TokenNeq && op != TokenContains && op != TokenNotContains {
			return fmt.Errorf("operator %q is not valid for string field %q (use =, !=, ~, or !~)", op, field)
		}

	case FieldPriority:
		// Priority supports all comparison operators

	case FieldDate:
		if op == TokenContains || op == TokenNotContains {
			return fmt.Errorf("operator %q is not valid for date field %q", op, field)
		}
	}

	return nil
}

// validateValue checks if a value is valid for a field type.
func validateValue(field string, fieldType FieldType, value Value) error {
	switch fieldType {
	case FieldBool:
		if value.Type != ValueBool {
			return fmt.Errorf("field %q requires a boolean value (true or false)", field)
		}

	case FieldPriority:
		switch value.Type {
		case ValuePriority:
			// Already validated by parser
		case ValueInt:
			if value.Int < 0 || value.Int > 4 {
				return fmt.Errorf("field %q requires priority 0-4, got %d", field, value.Int)
			}
		default:
			return fmt.Errorf("field %q requires a priority value (P0-P4 or 0-4), got %q", field, value.Raw)
		}

	case FieldDate:
		if value.Type != ValueDate {
			return fmt.Errorf("field %q requires a date value (today, yesterday, -Nd, or ISO date), got %q", field, value.Raw)
		}

	case FieldEnum:
		switch field {
		case "type":
			if !ValidTypeValues[value.String] {
				return fmt.Errorf("invalid value %q for field %q (valid: bug, feature, task, epic, chore)", value.String, field)
			}
		case "status":
			if !ValidStatusValues[strings.ToLower(value.String)] {
				return fmt.Errorf("invalid value %q for field %q (valid: open, in_progress, blocked, deferred, pinned, hooked, review, closed, tombstone)", value.String, field)
			}
		}

	case FieldString:
		// Any string value is valid
	}

	return nil
}

// validateOrderField checks if a field can be used in ORDER BY.
func validateOrderField(field string, validFields map[string]FieldType) error {
	_, ok := validFields[field]
	if !ok {
		return fmt.Errorf("unknown field in ORDER BY: %q (valid: %s)", field, fieldNamesFromMap(validFields))
	}
	return nil
}

// fieldNamesFromMap returns a comma-separated list of field names from the given map.
func fieldNamesFromMap(fields map[string]FieldType) string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}
