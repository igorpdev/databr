package emprego

import "testing"

func TestUFCodeToSigla(t *testing.T) {
	tests := []struct{ code, want string }{
		{"11", "RO"}, {"12", "AC"}, {"13", "AM"}, {"14", "RR"},
		{"15", "PA"}, {"16", "AP"}, {"17", "TO"},
		{"21", "MA"}, {"22", "PI"}, {"23", "CE"}, {"24", "RN"},
		{"25", "PB"}, {"26", "PE"}, {"27", "AL"}, {"28", "SE"}, {"29", "BA"},
		{"31", "MG"}, {"32", "ES"}, {"33", "RJ"}, {"35", "SP"},
		{"41", "PR"}, {"42", "SC"}, {"43", "RS"},
		{"50", "MS"}, {"51", "MT"}, {"52", "GO"}, {"53", "DF"},
		{"99", ""}, {"00", ""}, {"", ""},
	}
	for _, tt := range tests {
		if got := ufCodeToSigla(tt.code); got != tt.want {
			t.Errorf("ufCodeToSigla(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestCNAEDivisaoToSecao(t *testing.T) {
	tests := []struct{ cnae4, wantSecao, wantDesc string }{
		{"0111", "A", "Agricultura, pecuária, produção florestal, pesca e aquicultura"},
		{"0910", "B", "Indústrias extrativas"},
		{"1061", "C", "Indústrias de transformação"},
		{"3500", "D", "Eletricidade e gás"},
		{"3600", "E", "Água, esgoto, atividades de gestão de resíduos e descontaminação"},
		{"4120", "F", "Construção"},
		{"4711", "G", "Comércio; reparação de veículos automotores e motocicletas"},
		{"4911", "H", "Transporte, armazenagem e correio"},
		{"5510", "I", "Alojamento e alimentação"},
		{"6201", "J", "Informação e comunicação"},
		{"6630", "K", "Atividades financeiras, de seguros e serviços relacionados"},
		{"6810", "L", "Atividades imobiliárias"},
		{"6911", "M", "Atividades profissionais, científicas e técnicas"},
		{"7711", "N", "Atividades administrativas e serviços complementares"},
		{"8411", "O", "Administração pública, defesa e seguridade social"},
		{"8511", "P", "Educação"},
		{"8610", "Q", "Saúde humana e serviços sociais"},
		{"9001", "R", "Artes, cultura, esporte e recreação"},
		{"9411", "S", "Outras atividades de serviços"},
		{"9700", "T", "Serviços domésticos"},
		{"9900", "U", "Organismos internacionais e outras instituições extraterritoriais"},
		{"0000", "", ""},
	}
	for _, tt := range tests {
		sec, desc := cnaeDivisaoToSecao(tt.cnae4)
		if sec != tt.wantSecao || desc != tt.wantDesc {
			t.Errorf("cnaeDivisaoToSecao(%q) = (%q,%q), want (%q,%q)", tt.cnae4, sec, desc, tt.wantSecao, tt.wantDesc)
		}
	}
}

func TestCNAESecaoDescricao(t *testing.T) {
	if got := cnaeSecaoDescricao("C"); got != "Indústrias de transformação" {
		t.Errorf("cnaeSecaoDescricao(C) = %q", got)
	}
	if got := cnaeSecaoDescricao("Z"); got != "" {
		t.Errorf("cnaeSecaoDescricao(Z) = %q, want empty", got)
	}
}

func TestParseBRDecimal(t *testing.T) {
	tests := []struct {
		s    string
		want float64
	}{
		{"1800,00", 1800.00},
		{"0,00", 0},
		{"1514.25", 1514.25},
		{".00", 0},
		{"", 0},
		{"2500", 2500},
		{"1.234,56", 1234.56},
	}
	for _, tt := range tests {
		if got := parseBRDecimal(tt.s); got != tt.want {
			t.Errorf("parseBRDecimal(%q) = %f, want %f", tt.s, got, tt.want)
		}
	}
}
