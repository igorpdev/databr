package emprego

import (
	"strings"
	"testing"
)

func TestAggregateCAGED(t *testing.T) {
	input := "competênciamov;região;uf;município;seção;subclasse;saldomovimentação;cbo2002ocupação;categoria;graudeinstrução;idade;horascontratuais;raçacor;sexo;tipoempregador;tipoestabelecimento;tipomovimentação;tipodedeficiência;indtrabintermitente;indtrabparcial;salário;tamestabjan;indicadoraprendiz;origemdainformação;competênciadec;indicadordeforadoprazo;unidadesaláriocódigo;valorsaláriofixo\n" +
		"202501;1;35;350010;C;1061901;1;413115;101;7;27;44,00;3;1;0;1;97;0;0;0;1800,00;6;0;1;202501;0;5;1800,00\n" +
		"202501;1;35;350010;C;1061901;-1;413115;101;7;42;44,00;3;1;0;1;40;0;0;0;1600,00;2;0;1;202501;0;5;1600,00\n" +
		"202501;1;35;350010;G;4784900;1;519110;101;7;30;44,00;3;1;0;1;97;0;0;0;2000,00;3;0;1;202501;0;5;2000,00\n" +
		"202501;1;33;330001;C;1061901;1;413115;101;7;25;44,00;3;1;0;1;97;0;0;0;2200,00;5;0;1;202501;0;5;2200,00\n"

	records, err := aggregateCAGED(strings.NewReader(input), "202501")
	if err != nil {
		t.Fatalf("aggregateCAGED error: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Source != "caged_emprego" {
		t.Errorf("Source = %q, want caged_emprego", rec.Source)
	}
	if rec.RecordKey != "202501" {
		t.Errorf("RecordKey = %q, want 202501", rec.RecordKey)
	}

	items, ok := rec.Data["items"].([]map[string]any)
	if !ok {
		t.Fatalf("Data['items'] is not []map[string]any, got %T", rec.Data["items"])
	}
	// 3 unique (UF, CNAE) combos: (SP,C), (SP,G), (RJ,C)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Check total field
	total, _ := rec.Data["total"].(int)
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}

	// Find SP+C item — should have 1 admission, 1 dismissal, saldo 0
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
	if spC["admissoes"] != 1 {
		t.Errorf("SP+C admissoes = %v, want 1", spC["admissoes"])
	}
	if spC["desligamentos"] != 1 {
		t.Errorf("SP+C desligamentos = %v, want 1", spC["desligamentos"])
	}
	if spC["saldo"] != 0 {
		t.Errorf("SP+C saldo = %v, want 0", spC["saldo"])
	}
	// salario_medio should be (1800+1600)/2 = 1700
	if sal, ok := spC["salario_medio"].(float64); !ok || sal != 1700.0 {
		t.Errorf("SP+C salario_medio = %v, want 1700.0", spC["salario_medio"])
	}
}

func TestAggregateCAGED_EmptyInput(t *testing.T) {
	input := "competênciamov;região;uf;município;seção;subclasse;saldomovimentação\n"
	records, err := aggregateCAGED(strings.NewReader(input), "202501")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	items, ok := records[0].Data["items"].([]map[string]any)
	if !ok {
		t.Fatalf("items type = %T", records[0].Data["items"])
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestAggregateCAGED_UnknownUF(t *testing.T) {
	input := "competênciamov;região;uf;município;seção;subclasse;saldomovimentação;cbo2002ocupação;categoria;graudeinstrução;idade;horascontratuais;raçacor;sexo;tipoempregador;tipoestabelecimento;tipomovimentação;tipodedeficiência;indtrabintermitente;indtrabparcial;salário;tamestabjan;indicadoraprendiz;origemdainformação;competênciadec;indicadordeforadoprazo;unidadesaláriocódigo;valorsaláriofixo\n" +
		"202501;1;99;000000;C;1061901;1;413115;101;7;27;44,00;3;1;0;1;97;0;0;0;1800,00;6;0;1;202501;0;5;1800,00\n"
	records, err := aggregateCAGED(strings.NewReader(input), "202501")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	items := records[0].Data["items"].([]map[string]any)
	// UF 99 is unknown -> should be skipped
	if len(items) != 0 {
		t.Errorf("expected 0 items (unknown UF skipped), got %d", len(items))
	}
}
