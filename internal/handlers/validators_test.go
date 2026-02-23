package handlers

import (
	"strings"
	"testing"
)

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
		{"1122233300018a", false},  // non-digit character
		{"11111111111111", false},  // all same digits
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
	tests := []struct {
		input string
		want  string
	}{
		{"O'Brien", "O''Brien"},
		{"normal", "normal"},
		{"'; DROP TABLE users;--", "'' DROP TABLE users"},
		{"test;injection", "testinjection"},
		{"a--b", "ab"},
	}
	for _, tt := range tests {
		if got := sanitizeOData(tt.input); got != tt.want {
			t.Errorf("sanitizeOData(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsValidOrgao(t *testing.T) {
	tests := []struct {
		orgao string
		valid bool
	}{
		{"26000", true},
		{"1", true},
		{"123456", true},
		{"1234567", false}, // too long
		{"", false},
		{"abc", false},
		{"26000a", false},
		{"26 000", false},
	}
	for _, tt := range tests {
		if got := isValidOrgao(tt.orgao); got != tt.valid {
			t.Errorf("isValidOrgao(%q) = %v, want %v", tt.orgao, got, tt.valid)
		}
	}
}

func TestIsValidMunicipio(t *testing.T) {
	tests := []struct {
		code  string
		valid bool
	}{
		{"3550308", true},  // São Paulo
		{"1302603", true},  // Manaus
		{"355030", true},   // 6 digits OK
		{"12345", false},   // too short
		{"12345678", false}, // too long
		{"abc", false},
	}
	for _, tt := range tests {
		if got := isValidMunicipio(tt.code); got != tt.valid {
			t.Errorf("isValidMunicipio(%q) = %v, want %v", tt.code, got, tt.valid)
		}
	}
}

func TestIsValidDateISO(t *testing.T) {
	tests := []struct {
		date  string
		valid bool
	}{
		{"2024-01-15", true},
		{"2024-12-31", true},
		{"2024-02-29", true}, // leap year
		{"2024-13-01", false},
		{"2024-1-1", false},
		{"20240101", false},
		{"not-a-date", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isValidDateISO(tt.date); got != tt.valid {
			t.Errorf("isValidDateISO(%q) = %v, want %v", tt.date, got, tt.valid)
		}
	}
}

func TestIsValidCNAE(t *testing.T) {
	tests := []struct {
		code  string
		valid bool
	}{
		{"62", true},      // 2 digits OK
		{"6201500", true}, // 7 digits OK
		{"1", false},      // too short
		{"12345678", false}, // too long
	}
	for _, tt := range tests {
		if got := isValidCNAE(tt.code); got != tt.valid {
			t.Errorf("isValidCNAE(%q) = %v, want %v", tt.code, got, tt.valid)
		}
	}
}

func TestIsValidSeriesCodigo(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"BM12_TJOVER12", true},
		{"PRECOS12_IPCA12", true},
		{"SCN10_TRIBFBCF10", true},
		{"AB", true},
		{"a", false},
		{"", false},
		{"ABC'DEF", false},
		{"A;DROP", false},
		{"AB CD", false},
		{"foo/bar", false},
		{strings.Repeat("A", 51), false},
	}
	for _, tt := range tests {
		if got := isValidSeriesCodigo(tt.input); got != tt.want {
			t.Errorf("isValidSeriesCodigo(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeQueryParam(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal", "normal"},
		{"with space", "with+space"},
		{"special&chars=yes", "special%26chars%3Dyes"},
	}
	for _, tt := range tests {
		if got := sanitizeQueryParam(tt.input); got != tt.want {
			t.Errorf("sanitizeQueryParam(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
