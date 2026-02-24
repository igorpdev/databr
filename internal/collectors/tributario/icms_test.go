package tributario

import (
	"testing"
)

func TestICMSProvider_GetInternalRate(t *testing.T) {
	p := NewICMSProvider()

	tests := []struct {
		uf      string
		modal   float64
		fcp     float64
		efetiva float64
		regiao  string
	}{
		{"SP", 18.0, 0, 18.0, "sudeste"},
		{"RJ", 22.0, 2.0, 24.0, "sudeste"},
		{"MA", 23.0, 0, 23.0, "nordeste"},
		{"AL", 20.0, 1.0, 21.0, "nordeste"},
		{"RS", 17.0, 0, 17.0, "sul"},
		{"AC", 19.0, 0, 19.0, "norte"},
		{"DF", 20.0, 0, 20.0, "centro-oeste"},
	}

	for _, tt := range tests {
		t.Run(tt.uf, func(t *testing.T) {
			rate, err := p.GetInternalRate(tt.uf)
			if err != nil {
				t.Fatalf("GetInternalRate(%s) error: %v", tt.uf, err)
			}
			if rate.AliquotaInterna != tt.modal {
				t.Errorf("AliquotaInterna = %v, want %v", rate.AliquotaInterna, tt.modal)
			}
			if rate.FCP != tt.fcp {
				t.Errorf("FCP = %v, want %v", rate.FCP, tt.fcp)
			}
			if rate.AliquotaEfetiva != tt.efetiva {
				t.Errorf("AliquotaEfetiva = %v, want %v", rate.AliquotaEfetiva, tt.efetiva)
			}
			if rate.Regiao != tt.regiao {
				t.Errorf("Regiao = %q, want %q", rate.Regiao, tt.regiao)
			}
		})
	}
}

func TestICMSProvider_GetInternalRate_CaseInsensitive(t *testing.T) {
	p := NewICMSProvider()
	rate, err := p.GetInternalRate("sp")
	if err != nil {
		t.Fatalf("GetInternalRate(sp) error: %v", err)
	}
	if rate.UF != "SP" {
		t.Errorf("UF = %q, want SP", rate.UF)
	}
}

func TestICMSProvider_GetInternalRate_Invalid(t *testing.T) {
	p := NewICMSProvider()
	_, err := p.GetInternalRate("ZZ")
	if err == nil {
		t.Error("expected error for invalid UF")
	}
}

func TestICMSProvider_GetInterstateRate_7Percent(t *testing.T) {
	p := NewICMSProvider()
	// SP (Sul/Sudeste) → MA (Nordeste) = 7%
	result, err := p.GetInterstateRate("SP", "MA")
	if err != nil {
		t.Fatalf("GetInterstateRate(SP, MA) error: %v", err)
	}
	if result.AliquotaInterestadual != 7.0 {
		t.Errorf("AliquotaInterestadual = %v, want 7.0", result.AliquotaInterestadual)
	}
	// DIFAL = destino efetiva (23%) - interestadual (7%) = 16%
	if result.DIFAL != 16.0 {
		t.Errorf("DIFAL = %v, want 16.0", result.DIFAL)
	}
	if result.AliquotaImportados != 4.0 {
		t.Errorf("AliquotaImportados = %v, want 4.0", result.AliquotaImportados)
	}
}

func TestICMSProvider_GetInterstateRate_12Percent(t *testing.T) {
	p := NewICMSProvider()
	// BA (Nordeste) → SP (Sul/Sudeste) = 12%
	result, err := p.GetInterstateRate("BA", "SP")
	if err != nil {
		t.Fatalf("GetInterstateRate(BA, SP) error: %v", err)
	}
	if result.AliquotaInterestadual != 12.0 {
		t.Errorf("AliquotaInterestadual = %v, want 12.0", result.AliquotaInterestadual)
	}
}

func TestICMSProvider_GetInterstateRate_ESException(t *testing.T) {
	p := NewICMSProvider()
	// ES is Sudeste but in "demais" group — ES → SP should be 12%, not 7%
	result, err := p.GetInterstateRate("ES", "SP")
	if err != nil {
		t.Fatalf("GetInterstateRate(ES, SP) error: %v", err)
	}
	if result.AliquotaInterestadual != 12.0 {
		t.Errorf("ES→SP: AliquotaInterestadual = %v, want 12.0 (ES exception)", result.AliquotaInterestadual)
	}

	// SP → ES should be 7% (SP is sul_sudeste, ES is demais)
	result2, err := p.GetInterstateRate("SP", "ES")
	if err != nil {
		t.Fatalf("GetInterstateRate(SP, ES) error: %v", err)
	}
	if result2.AliquotaInterestadual != 7.0 {
		t.Errorf("SP→ES: AliquotaInterestadual = %v, want 7.0", result2.AliquotaInterestadual)
	}
}

func TestICMSProvider_GetInterstateRate_SameState(t *testing.T) {
	p := NewICMSProvider()
	result, err := p.GetInterstateRate("SP", "SP")
	if err != nil {
		t.Fatalf("GetInterstateRate(SP, SP) error: %v", err)
	}
	if result.DIFAL != 0 {
		t.Errorf("DIFAL for same state = %v, want 0", result.DIFAL)
	}
}

func TestICMSProvider_GetAllRates(t *testing.T) {
	p := NewICMSProvider()
	rates := p.GetAllRates()
	if len(rates) != 27 {
		t.Errorf("GetAllRates() returned %d states, want 27", len(rates))
	}
	// Verify alphabetical order.
	if rates[0].UF != "AC" {
		t.Errorf("first UF = %q, want AC", rates[0].UF)
	}
	if rates[26].UF != "TO" {
		t.Errorf("last UF = %q, want TO", rates[26].UF)
	}
}

func TestValidUFs(t *testing.T) {
	if !ValidUFs["SP"] {
		t.Error("SP should be valid")
	}
	if ValidUFs["ZZ"] {
		t.Error("ZZ should not be valid")
	}
	if len(ValidUFs) != 27 {
		t.Errorf("ValidUFs has %d entries, want 27", len(ValidUFs))
	}
}
