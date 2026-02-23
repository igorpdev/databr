package handlers

import "testing"

func TestIsValidCNPJ(t *testing.T) {
	tests := []struct {
		cnpj  string
		valid bool
	}{
		{"11222333000181", true},
		{"11444777000161", true},
		{"11222333000182", false},  // wrong check digit
		{"00000000000000", false},  // all zeros
		{"1122233300018", false},   // too short
		{"112223330001811", false}, // too long
	}
	for _, tt := range tests {
		if got := isValidCNPJ(tt.cnpj); got != tt.valid {
			t.Errorf("isValidCNPJ(%s) = %v, want %v", tt.cnpj, got, tt.valid)
		}
	}
}

func TestIsValidUF(t *testing.T) {
	if !isValidUF("SP") {
		t.Error("SP should be valid")
	}
	if !isValidUF("sp") {
		t.Error("sp (lowercase) should be valid")
	}
	if isValidUF("XX") {
		t.Error("XX should be invalid")
	}
	if isValidUF("") {
		t.Error("empty should be invalid")
	}
}

func TestIsValidTicker(t *testing.T) {
	if !isValidTicker("PETR4") {
		t.Error("PETR4 should be valid")
	}
	if !isValidTicker("BOVA11") {
		t.Error("BOVA11 should be valid")
	}
	if isValidTicker("AB") {
		t.Error("AB too short")
	}
	if isValidTicker("ABCDEFGH") {
		t.Error("ABCDEFGH too long")
	}
	if isValidTicker("PE-R4") {
		t.Error("PE-R4 has invalid char")
	}
}

func TestIsValidCPFOrCNPJ(t *testing.T) {
	if !isValidCPFOrCNPJ("12345678901") {
		t.Error("11 digits should be valid CPF")
	}
	if !isValidCPFOrCNPJ("12345678000190") {
		t.Error("14 digits should be valid CNPJ")
	}
	if isValidCPFOrCNPJ("123456789") {
		t.Error("9 digits should be invalid")
	}
}

func TestSanitizeOData(t *testing.T) {
	if got := sanitizeOData("O'Brien"); got != "O''Brien" {
		t.Errorf("sanitizeOData(O'Brien) = %q, want O''Brien", got)
	}
	if got := sanitizeOData("normal"); got != "normal" {
		t.Errorf("sanitizeOData(normal) = %q, want normal", got)
	}
}
