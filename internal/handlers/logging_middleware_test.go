package handlers

import "testing"

func TestMaskPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		// Route patterns with {param} should be returned as-is
		{"/v1/empresas/{cnpj}", "/v1/empresas/{cnpj}"},
		{"/v1/judicial/processos/{doc}", "/v1/judicial/processos/{doc}"},
		// Actual CNPJ values (14 digits) should be masked
		{"/v1/empresas/12345678000190", "/v1/empresas/1234**********"},
		// Actual CPF values (11 digits) should be masked
		{"/v1/compliance/12345678901", "/v1/compliance/123********"},
		// Short values should not be masked
		{"/v1/bcb/selic", "/v1/bcb/selic"},
		{"/v1/ibge/municipio/3550308", "/v1/ibge/municipio/3550308"},
		// Health check
		{"/health", "/health"},
	}
	for _, tt := range tests {
		if got := maskPath(tt.path); got != tt.want {
			t.Errorf("maskPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
