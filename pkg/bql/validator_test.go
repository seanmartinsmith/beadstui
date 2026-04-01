package bql

import "testing"

func TestValidate_ValidQueries(t *testing.T) {
	queries := []string{
		"status = open",
		"priority < P2",
		"type = bug",
		"title ~ auth",
		"blocked = true",
		"label = urgent",
		"created_at > -7d",
		"status = open and priority < P2",
		"type in (bug, task)",
		"not blocked = true",
		"(type = bug or type = task) and priority = P0",
		"status = open order by priority asc",
	}

	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			query, err := Parse(q)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if err := Validate(query); err != nil {
				t.Errorf("Validate(%q): %v", q, err)
			}
		})
	}
}

func TestValidate_InvalidField(t *testing.T) {
	query, err := Parse("nonexistent = value")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Validate(query); err == nil {
		t.Error("expected error for unknown field")
	}
}

func TestValidate_InvalidOperatorForBool(t *testing.T) {
	query, err := Parse("blocked > true")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Validate(query); err == nil {
		t.Error("expected error for > on bool field")
	}
}

func TestValidate_InvalidOperatorForEnum(t *testing.T) {
	query, err := Parse("status > open")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Validate(query); err == nil {
		t.Error("expected error for > on enum field")
	}
}

func TestValidate_InvalidTypeValue(t *testing.T) {
	query, err := Parse("type = nonexistent")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Validate(query); err == nil {
		t.Error("expected error for invalid type value")
	}
}

func TestValidate_InvalidPriorityValue(t *testing.T) {
	query, err := Parse("priority = 99")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := Validate(query); err == nil {
		t.Error("expected error for priority=99")
	}
}

func TestValidate_InOnBoolField(t *testing.T) {
	query, err := Parse("blocked in (true, false)")
	if err != nil {
		// Parser may reject this before validator
		return
	}
	if err := Validate(query); err == nil {
		t.Error("expected error for IN on bool field")
	}
}

func TestValidate_CustomFields(t *testing.T) {
	customFields := map[string]FieldType{
		"custom_field": FieldString,
	}
	query, err := Parse("custom_field = value")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := ValidateWithFields(query, customFields); err != nil {
		t.Errorf("ValidateWithFields: %v", err)
	}

	// Standard field should fail with custom fields
	query2, _ := Parse("status = open")
	if err := ValidateWithFields(query2, customFields); err == nil {
		t.Error("expected error for status with custom fields only")
	}
}
