package emprego_test

import (
	"testing"

	"github.com/databr/api/internal/collectors/emprego"
)

func TestCAGEDCollector_Source(t *testing.T) {
	c := emprego.NewCAGEDCollector("")
	if got := c.Source(); got != "caged_emprego" {
		t.Errorf("Source() = %q, want caged_emprego", got)
	}
}

func TestCAGEDCollector_Schedule(t *testing.T) {
	c := emprego.NewCAGEDCollector("")
	if got := c.Schedule(); got != "0 12 1 * *" {
		t.Errorf("Schedule() = %q, want %q", got, "0 12 1 * *")
	}
}
