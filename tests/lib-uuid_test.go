package main

import (
	"regexp"
	"testing"
)

func init() {
	buildStandardLib()
}

func TestUuidGenerate(t *testing.T) {
	// Generate a UUID and check format
	u, err := stdlib["uuid_generate"]("", 0, nil)
	if err != nil {
		t.Fatalf("uuid_generate() failed: %v", err)
	}

	us := u.(string)
	if len(us) != 36 {
		t.Errorf("uuid length expected 36, got %d", len(us))
	}

	// Check dash positions
	if us[8] != '-' || us[13] != '-' || us[18] != '-' || us[23] != '-' {
		t.Errorf("uuid dash positions incorrect: %s", us)
	}

	// Check version 4
	if us[14] != '4' {
		t.Errorf("uuid version expected 4, got %c", us[14])
	}

	// Check variant (8-9a-b)
	vc := us[19]
	if vc != '8' && vc != '9' && vc != 'a' && vc != 'b' {
		t.Errorf("uuid variant unexpected: %c", vc)
	}

	// Check all other chars are hex
	hexRe := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !hexRe.MatchString(us) {
		t.Errorf("uuid does not match expected format: %s", us)
	}
}

func TestUuidGenerateUnique(t *testing.T) {
	// Generate many UUIDs and check uniqueness
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		u, err := stdlib["uuid_generate"]("", 0, nil)
		if err != nil {
			t.Fatalf("uuid_generate() failed: %v", err)
		}
		us := u.(string)
		if seen[us] {
			t.Fatalf("duplicate uuid generated: %s", us)
		}
		seen[us] = true
	}
}

func TestUuidParse(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000"},
		{"550e8400e29b41d4a716446655440000", "550e8400-e29b-41d4-a716-446655440000"},
		{"550E8400-E29B-41D4-A716-446655440000", "550e8400-e29b-41d4-a716-446655440000"},
		{"550E8400E29B41D4A716446655440000", "550e8400-e29b-41d4-a716-446655440000"},
	}

	for _, tc := range tests {
		got, err := stdlib["uuid_parse"]("", 0, nil, tc.input)
		if err != nil {
			t.Fatalf("uuid_parse(%q) failed: %v", tc.input, err)
		}
		if got.(string) != tc.expected {
			t.Errorf("uuid_parse(%q) = %q, want %q", tc.input, got.(string), tc.expected)
		}
	}

	// Invalid inputs should return empty string
	invalid := []string{"not-a-uuid", "550e8400", "550e8400-e29b-41d4-a716-44665544000g", ""}
	for _, in := range invalid {
		got, err := stdlib["uuid_parse"]("", 0, nil, in)
		if err != nil {
			t.Fatalf("uuid_parse(%q) failed: %v", in, err)
		}
		if got.(string) != "" {
			t.Errorf("uuid_parse(%q) = %q, want empty string", in, got.(string))
		}
	}
}

func TestUuidValidate(t *testing.T) {
	valid := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"00000000-0000-0000-0000-000000000000",
		"ffffffff-ffff-ffff-ffff-ffffffffffff",
	}
	for _, v := range valid {
		got, err := stdlib["uuid_validate"]("", 0, nil, v)
		if err != nil {
			t.Fatalf("uuid_validate(%q) failed: %v", v, err)
		}
		if !got.(bool) {
			t.Errorf("uuid_validate(%q) = false, want true", v)
		}
	}

	invalid := []string{
		"550e8400e29b41d4a716446655440000",      // Missing dashes
		"550e8400-e29b-41d4-a716-44665544000",   // Too short
		"550e8400-e29b-41d4-a716-4466554400000", // Too long
		"550e8400-e29b-41d4-a716-44665544000g",  // Invalid hex
		"not-a-uuid",
		"",
		"550e8400-e29b-41d4-a716_446655440000", // Wrong separator
	}
	for _, v := range invalid {
		got, err := stdlib["uuid_validate"]("", 0, nil, v)
		if err != nil {
			t.Fatalf("uuid_validate(%q) failed: %v", v, err)
		}
		if got.(bool) {
			t.Errorf("uuid_validate(%q) = true, want false", v)
		}
	}
}

func TestUuidRoundTrip(t *testing.T) {
	// Generate a UUID and validate it
	u, err := stdlib["uuid_generate"]("", 0, nil)
	if err != nil {
		t.Fatalf("uuid_generate() failed: %v", err)
	}
	us := u.(string)

	// Parse it
	parsed, err := stdlib["uuid_parse"]("", 0, nil, us)
	if err != nil {
		t.Fatalf("uuid_parse(%q) failed: %v", us, err)
	}
	if parsed.(string) != us {
		t.Errorf("uuid_parse(generate) = %q, want %q", parsed.(string), us)
	}

	// Validate it
	valid, err := stdlib["uuid_validate"]("", 0, nil, us)
	if err != nil {
		t.Fatalf("uuid_validate(%q) failed: %v", us, err)
	}
	if !valid.(bool) {
		t.Errorf("uuid_validate(generate) = false, want true")
	}
}
