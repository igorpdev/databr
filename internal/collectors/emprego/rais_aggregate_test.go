package emprego

import (
	"strings"
	"testing"
)

func TestAggregateRAIS(t *testing.T) {
	// Real RAIS format: comma-delimited, 61 columns, quoted headers
	// We use simplified headers matching by substring
	header := `"Bairros SP","Bairros Fort","Bairros RJ","Causa Af 1","Causa Af 2","Causa Af 3","Motivo Desl","CBO 2002","CNAE 2.0 Classe","CNAE 95","Distritos SP","Ind Vinculo Ativo 31/12","Faixa Etaria","Faixa Rem Med","Faixa Hora","Faixa Rem Dez","Faixa Tempo","Escolaridade","Qtd Hora","Idade","Ind CEI","Ind SIMPLES","Mes Adm","Mes Desl","Municipio Trab","Municipio","Nacional","Nat Jurid","Ind Defic","Qtd Dias Afast","Raca Cor","Regiao DF","Vl Rem Dez Nom","Vl Rem Dez SM","Vl Rem Media Nom","Vl Rem Media SM","CNAE Sub","Sexo","Tam Estab","Tempo Emp","Tipo Adm","Tipo Estab","Nome Estab","Tipo Defic","Tipo Vinc","IBGE Sub","Jan","Fev","Mar","Abr","Mai","Jun","Jul","Ago","Set","Out","Nov","Ano Cheg","Ind Interm","Ind Parcial","Ind Abandon"` + "\n"
	// Row 1: SP (350010→35→SP), CNAE 1061→C, active, rem 1800.44
	row1 := `999997,999997,999997,999,999,999,0,514320,1061,6712,999997,1,7,99,6,99,5,4,44,64,0,1,0,0,350010,350010,10,2062,0,0,8,999997,.00,3.5,1800.44,2.0,1061000,1,8,34.3,0,1,"CNPJ",0,10,18,0,0,0,0,0,0,0,0,0,0,0,1198,0,0,0` + "\n"
	// Row 2: RJ (330001→33→RJ), CNAE 4711→G, inactive, rem 2500.00
	row2 := `999997,999997,999997,999,999,999,0,411005,4711,6712,999997,0,5,99,6,99,3,6,44,35,0,1,0,0,330001,330001,10,2062,0,0,2,999997,.00,2.0,2500.00,3.0,4711300,2,5,12.1,0,1,"CNPJ",0,10,18,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0` + "\n"
	// Row 3: SP, CNAE 1061→C, active, rem 2200.00
	row3 := `999997,999997,999997,999,999,999,0,514320,1061,6712,999997,1,7,99,6,99,5,4,44,40,0,1,0,0,350010,350010,10,2062,0,0,3,999997,.00,4.0,2200.00,2.5,1061000,1,8,20.0,0,1,"CNPJ",0,10,18,0,0,0,0,0,0,0,0,0,0,0,1198,0,0,0` + "\n"

	input := header + row1 + row2 + row3
	records, err := aggregateRAIS(strings.NewReader(input), 2024)
	if err != nil {
		t.Fatalf("aggregateRAIS error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Source != "rais_emprego" {
		t.Errorf("Source = %q, want rais_emprego", rec.Source)
	}
	if rec.RecordKey != "2024" {
		t.Errorf("RecordKey = %q, want 2024", rec.RecordKey)
	}

	items := rec.Data["items"].([]map[string]any)
	// 2 unique combos: (SP,C), (RJ,G)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", len(items), items)
	}

	// Find SP+C: 2 rows, 2 active, avg rem = (1800.44+2200)/2 = 2000.22
	var spC map[string]any
	for _, item := range items {
		if item["uf"] == "SP" && item["cnae_secao"] == "C" {
			spC = item
			break
		}
	}
	if spC == nil {
		t.Fatal("missing SP+C item")
	}
	if spC["vinculos_total"] != 2 {
		t.Errorf("SP+C vinculos_total = %v, want 2", spC["vinculos_total"])
	}
	if spC["ativos_dez31"] != 2 {
		t.Errorf("SP+C ativos_dez31 = %v, want 2", spC["ativos_dez31"])
	}
	if rem, ok := spC["remuneracao_media"].(float64); !ok || rem != 2000.22 {
		t.Errorf("SP+C remuneracao_media = %v, want 2000.22", spC["remuneracao_media"])
	}

	// Find RJ+G: 1 row, 0 active, rem 2500.00
	var rjG map[string]any
	for _, item := range items {
		if item["uf"] == "RJ" && item["cnae_secao"] == "G" {
			rjG = item
			break
		}
	}
	if rjG == nil {
		t.Fatal("missing RJ+G item")
	}
	if rjG["vinculos_total"] != 1 {
		t.Errorf("RJ+G vinculos_total = %v, want 1", rjG["vinculos_total"])
	}
	if rjG["ativos_dez31"] != 0 {
		t.Errorf("RJ+G ativos_dez31 = %v, want 0", rjG["ativos_dez31"])
	}
}

func TestAggregateRAIS_EmptyInput(t *testing.T) {
	header := `"CNAE 2.0 Classe","Ind Vinculo Ativo 31/12","Municipio Trab","Vl Rem Media Nom"` + "\n"
	records, err := aggregateRAIS(strings.NewReader(header), 2024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	items := records[0].Data["items"].([]map[string]any)
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestAggregateRAIS_UnknownMunicipality(t *testing.T) {
	header := `"Bairros SP","Bairros Fort","Bairros RJ","Causa Af 1","Causa Af 2","Causa Af 3","Motivo Desl","CBO 2002","CNAE 2.0 Classe","CNAE 95","Distritos SP","Ind Vinculo Ativo 31/12","Faixa Etaria","Faixa Rem Med","Faixa Hora","Faixa Rem Dez","Faixa Tempo","Escolaridade","Qtd Hora","Idade","Ind CEI","Ind SIMPLES","Mes Adm","Mes Desl","Municipio Trab","Municipio","Nacional","Nat Jurid","Ind Defic","Qtd Dias Afast","Raca Cor","Regiao DF","Vl Rem Dez Nom","Vl Rem Dez SM","Vl Rem Media Nom","Vl Rem Media SM","CNAE Sub","Sexo","Tam Estab","Tempo Emp","Tipo Adm","Tipo Estab","Nome Estab","Tipo Defic","Tipo Vinc","IBGE Sub","Jan","Fev","Mar","Abr","Mai","Jun","Jul","Ago","Set","Out","Nov","Ano Cheg","Ind Interm","Ind Parcial","Ind Abandon"` + "\n"
	// Municipality 999999 → UF "99" → unknown, should be skipped
	row := `999997,999997,999997,999,999,999,0,514320,1061,6712,999997,1,7,99,6,99,5,4,44,64,0,1,0,0,999999,999999,10,2062,0,0,8,999997,.00,3.5,1800.44,2.0,1061000,1,8,34.3,0,1,"CNPJ",0,10,18,0,0,0,0,0,0,0,0,0,0,0,1198,0,0,0` + "\n"

	records, err := aggregateRAIS(strings.NewReader(header+row), 2024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	items := records[0].Data["items"].([]map[string]any)
	if len(items) != 0 {
		t.Errorf("expected 0 items (unknown municipality skipped), got %d", len(items))
	}
}
