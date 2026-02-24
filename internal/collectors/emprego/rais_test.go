package emprego_test

import (
	"testing"

	"github.com/databr/api/internal/collectors/emprego"
)

func TestRAISCollector_Source(t *testing.T) {
	c := emprego.NewRAISCollector("")
	if got := c.Source(); got != "rais_emprego" {
		t.Errorf("Source() = %q, want rais_emprego", got)
	}
}

func TestRAISCollector_Schedule(t *testing.T) {
	c := emprego.NewRAISCollector("")
	if got := c.Schedule(); got != "0 3 1 3 *" {
		t.Errorf("Schedule() = %q, want %q", got, "0 3 1 3 *")
	}
}
