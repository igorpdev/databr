package tributario

import (
	"fmt"
	"strings"
)

// ICMSRate holds the internal ICMS rate for a Brazilian state.
type ICMSRate struct {
	UF              string  `json:"uf"`
	Nome            string  `json:"nome"`
	AliquotaInterna float64 `json:"aliquota_interna"`
	FCP             float64 `json:"fcp"`
	AliquotaEfetiva float64 `json:"aliquota_efetiva"`
	Regiao          string  `json:"regiao"`
	GrupoInter      string  `json:"grupo_interestadual"`
}

// InterstateRate holds the result of an interstate ICMS calculation.
type InterstateRate struct {
	Origem                map[string]any `json:"origem"`
	Destino               map[string]any `json:"destino"`
	AliquotaInterestadual float64        `json:"aliquota_interestadual"`
	AliquotaImportados    float64        `json:"aliquota_importados"`
	DIFAL                 float64        `json:"difal"`
	Regra                 string         `json:"regra"`
}

// ICMSProvider serves static ICMS rate data.
// Rates reflect 2026 values (CONFAZ / state legislation).
type ICMSProvider struct{}

// NewICMSProvider creates a new ICMS provider.
func NewICMSProvider() *ICMSProvider {
	return &ICMSProvider{}
}

// Internal ICMS rates per state (2026).
// Source: CONFAZ + state legislation, verified via tributodevido.com.br (2026-02-24).
var internalRates = map[string]struct {
	nome    string
	modal   float64
	fcp     float64
	regiao  string
	grupo   string // "sul_sudeste" or "demais" (for interstate rate calculation)
}{
	"AC": {"Acre", 19.0, 0, "norte", "demais"},
	"AL": {"Alagoas", 20.0, 1.0, "nordeste", "demais"},
	"AM": {"Amazonas", 20.0, 0, "norte", "demais"},
	"AP": {"Amapá", 18.0, 0, "norte", "demais"},
	"BA": {"Bahia", 20.5, 0, "nordeste", "demais"},
	"CE": {"Ceará", 20.0, 0, "nordeste", "demais"},
	"DF": {"Distrito Federal", 20.0, 0, "centro-oeste", "demais"},
	"ES": {"Espírito Santo", 17.0, 0, "sudeste", "demais"}, // ES is exception: Sudeste but applies 12%
	"GO": {"Goiás", 19.0, 0, "centro-oeste", "demais"},
	"MA": {"Maranhão", 23.0, 0, "nordeste", "demais"},
	"MT": {"Mato Grosso", 17.0, 0, "centro-oeste", "demais"},
	"MS": {"Mato Grosso do Sul", 19.0, 0, "centro-oeste", "demais"},
	"MG": {"Minas Gerais", 18.0, 0, "sudeste", "sul_sudeste"},
	"PA": {"Pará", 19.0, 0, "norte", "demais"},
	"PB": {"Paraíba", 20.0, 0, "nordeste", "demais"},
	"PR": {"Paraná", 19.5, 0, "sul", "sul_sudeste"},
	"PE": {"Pernambuco", 20.5, 0, "nordeste", "demais"},
	"PI": {"Piauí", 22.5, 0, "nordeste", "demais"},
	"RJ": {"Rio de Janeiro", 22.0, 2.0, "sudeste", "sul_sudeste"},
	"RN": {"Rio Grande do Norte", 20.0, 0, "nordeste", "demais"},
	"RS": {"Rio Grande do Sul", 17.0, 0, "sul", "sul_sudeste"},
	"RO": {"Rondônia", 19.5, 0, "norte", "demais"},
	"RR": {"Roraima", 20.0, 0, "norte", "demais"},
	"SC": {"Santa Catarina", 17.0, 0, "sul", "sul_sudeste"},
	"SP": {"São Paulo", 18.0, 0, "sudeste", "sul_sudeste"},
	"SE": {"Sergipe", 20.0, 0, "nordeste", "demais"},
	"TO": {"Tocantins", 20.0, 0, "norte", "demais"},
}

// ValidUFs is the set of valid Brazilian state codes.
var ValidUFs = func() map[string]bool {
	m := make(map[string]bool, len(internalRates))
	for uf := range internalRates {
		m[uf] = true
	}
	return m
}()

// GetInternalRate returns the internal ICMS rate for a state.
func (p *ICMSProvider) GetInternalRate(uf string) (*ICMSRate, error) {
	uf = strings.ToUpper(strings.TrimSpace(uf))
	info, ok := internalRates[uf]
	if !ok {
		return nil, fmt.Errorf("icms: UF '%s' not found", uf)
	}
	return &ICMSRate{
		UF:              uf,
		Nome:            info.nome,
		AliquotaInterna: info.modal,
		FCP:             info.fcp,
		AliquotaEfetiva: info.modal + info.fcp,
		Regiao:          info.regiao,
		GrupoInter:      info.grupo,
	}, nil
}

// GetInterstateRate calculates the interstate ICMS rate between two states.
func (p *ICMSProvider) GetInterstateRate(origem, destino string) (*InterstateRate, error) {
	origem = strings.ToUpper(strings.TrimSpace(origem))
	destino = strings.ToUpper(strings.TrimSpace(destino))

	origInfo, ok := internalRates[origem]
	if !ok {
		return nil, fmt.Errorf("icms: origin UF '%s' not found", origem)
	}
	destInfo, ok := internalRates[destino]
	if !ok {
		return nil, fmt.Errorf("icms: destination UF '%s' not found", destino)
	}

	// Same state = internal rate, no interstate.
	if origem == destino {
		return &InterstateRate{
			Origem:  map[string]any{"uf": origem, "aliquota_interna": origInfo.modal + origInfo.fcp},
			Destino: map[string]any{"uf": destino, "aliquota_interna": destInfo.modal + destInfo.fcp},
			AliquotaInterestadual: origInfo.modal + origInfo.fcp,
			AliquotaImportados:    4.0,
			DIFAL:                 0,
			Regra:                 "Operação interna (mesmo estado)",
		}, nil
	}

	// Interstate rate rules:
	// - 7%: origin Sul/Sudeste (except ES) → destination N/NE/CO/ES
	// - 12%: all other interstate
	// - 4%: imported products (always, regardless of origin/destination)
	var rate float64
	var regra string

	if origInfo.grupo == "sul_sudeste" && destInfo.grupo == "demais" {
		rate = 7.0
		regra = "Sul/Sudeste (exceto ES) → Norte/Nordeste/Centro-Oeste/ES: 7%"
	} else {
		rate = 12.0
		regra = "Demais operações interestaduais: 12%"
	}

	difal := (destInfo.modal + destInfo.fcp) - rate

	return &InterstateRate{
		Origem:  map[string]any{"uf": origem, "aliquota_interna": origInfo.modal + origInfo.fcp},
		Destino: map[string]any{"uf": destino, "aliquota_interna": destInfo.modal + destInfo.fcp},
		AliquotaInterestadual: rate,
		AliquotaImportados:    4.0,
		DIFAL:                 difal,
		Regra:                 regra,
	}, nil
}

// GetAllRates returns internal ICMS rates for all 27 states.
func (p *ICMSProvider) GetAllRates() []ICMSRate {
	// Return in alphabetical order by UF.
	ufs := []string{
		"AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO",
		"MA", "MT", "MS", "MG", "PA", "PB", "PR", "PE", "PI",
		"RJ", "RN", "RS", "RO", "RR", "SC", "SP", "SE", "TO",
	}
	rates := make([]ICMSRate, 0, len(ufs))
	for _, uf := range ufs {
		info := internalRates[uf]
		rates = append(rates, ICMSRate{
			UF:              uf,
			Nome:            info.nome,
			AliquotaInterna: info.modal,
			FCP:             info.fcp,
			AliquotaEfetiva: info.modal + info.fcp,
			Regiao:          info.regiao,
			GrupoInter:      info.grupo,
		})
	}
	return rates
}
