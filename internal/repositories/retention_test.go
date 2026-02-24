package repositories

import (
	"testing"
	"time"
)

func TestRetentionPolicies_AllHaveRequiredFields(t *testing.T) {
	policies := RetentionPolicies()
	if len(policies) == 0 {
		t.Fatal("expected at least one retention policy")
	}

	seen := make(map[string]bool)
	for _, p := range policies {
		if p.Source == "" {
			t.Error("policy has empty Source")
		}
		if p.MaxAge <= 0 {
			t.Errorf("policy %s has non-positive MaxAge: %v", p.Source, p.MaxAge)
		}
		if p.Category == "" {
			t.Errorf("policy %s has empty Category", p.Source)
		}
		if seen[p.Source] {
			t.Errorf("duplicate policy for source: %s", p.Source)
		}
		seen[p.Source] = true
	}
}

func TestRetentionPolicies_CategoriesAreValid(t *testing.T) {
	validCategories := map[string]bool{
		"time_series":  true,
		"accumulating": true,
		"on_demand":    true,
	}

	for _, p := range RetentionPolicies() {
		if !validCategories[p.Category] {
			t.Errorf("policy %s has invalid category: %s", p.Source, p.Category)
		}
	}
}

func TestRetentionPolicies_MaxAgesAreReasonable(t *testing.T) {
	day := 24 * time.Hour

	for _, p := range RetentionPolicies() {
		if p.MaxAge < 1*day {
			t.Errorf("policy %s MaxAge too short: %v (min 1 day)", p.Source, p.MaxAge)
		}
		if p.MaxAge > 366*day {
			t.Errorf("policy %s MaxAge too long: %v (max 1 year)", p.Source, p.MaxAge)
		}
	}
}

func TestRetentionPolicies_OnDemandSourcesHaveShortTTL(t *testing.T) {
	week := 7 * 24 * time.Hour

	for _, p := range RetentionPolicies() {
		if p.Category == "on_demand" && p.MaxAge > week {
			t.Errorf("on_demand source %s should have MaxAge <= 7 days, got %v", p.Source, p.MaxAge)
		}
	}
}

func TestRetentionPolicies_SnapshotSourcesNotListed(t *testing.T) {
	// Snapshot sources should NOT have retention policies (they're bounded by upsert)
	snapshotSources := []string{
		"camara_deputados", "senado_senadores", "tesouro_titulos",
		"ans_operadoras", "bcb_focus", "bcb_pix", "bcb_taxas_credito",
		"inep_censo_escolar", "rais_emprego", "anac_rab",
		"ibge_populacao", "b3_ibovespa", "tesouro_siconfi",
		"inpe_prodes", "mapbiomas_cobertura",
	}

	policySet := make(map[string]bool)
	for _, p := range RetentionPolicies() {
		policySet[p.Source] = true
	}

	for _, s := range snapshotSources {
		if policySet[s] {
			t.Errorf("snapshot source %s should NOT have a retention policy (bounded by upsert)", s)
		}
	}
}

func TestRawDataMaxAge(t *testing.T) {
	expected := 7 * 24 * time.Hour
	if RawDataMaxAge != expected {
		t.Errorf("RawDataMaxAge = %v, want %v", RawDataMaxAge, expected)
	}
}
