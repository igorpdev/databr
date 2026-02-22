package x402_test

import (
	"testing"

	"github.com/databr/api/internal/x402"
)

func TestPricing_KnownRoute(t *testing.T) {
	tests := []struct {
		route    string
		wantUSDC string
	}{
		{"/v1/empresas/{cnpj}", "0.001"},
		{"/v1/empresas/{cnpj}/compliance", "0.003"},
		{"/v1/mercado/acoes/{ticker}", "0.002"},
		{"/v1/mercado/fundos/{cnpj}", "0.005"},
		{"/v1/bcb/cambio/{moeda}", "0.001"},
		{"/v1/bcb/selic", "0.001"},
		{"/v1/compliance/{cnpj}", "0.005"},
		{"/v1/economia/ipca", "0.001"},
		{"/v1/economia/pib", "0.001"},
		{"/v1/judicial/processos/{doc}", "0.010"},
		{"/v1/dou/busca", "0.003"},
	}

	for _, tt := range tests {
		t.Run(tt.route, func(t *testing.T) {
			got, ok := x402.PriceFor(tt.route)
			if !ok {
				t.Fatalf("PriceFor(%q) = not found, want %s USDC", tt.route, tt.wantUSDC)
			}
			if got != tt.wantUSDC {
				t.Errorf("PriceFor(%q) = %q, want %q", tt.route, got, tt.wantUSDC)
			}
		})
	}
}

func TestPricing_UnknownRoute(t *testing.T) {
	_, ok := x402.PriceFor("/v1/unknown/endpoint")
	if ok {
		t.Error("PriceFor unknown route should return ok=false")
	}
}

func TestPricing_ContextAdditional(t *testing.T) {
	got, ok := x402.PriceFor("/v1/bcb/selic")
	if !ok {
		t.Fatal("expected bcb/selic to be priced")
	}
	withCtx := x402.AddContextPrice(got)
	if withCtx == got {
		t.Error("AddContextPrice should return a higher price than base")
	}
	if withCtx != "0.002" {
		t.Errorf("AddContextPrice(0.001) = %q, want 0.002", withCtx)
	}
}
