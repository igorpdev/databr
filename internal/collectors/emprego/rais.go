// Package emprego implements collectors for Brazilian employment data sources (RAIS, CAGED).
package emprego

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/databr/api/internal/domain"
	"golang.org/x/text/encoding/charmap"
)

var raisRegionFiles = []string{
	"RAIS_VINC_PUB_SP",
	"RAIS_VINC_PUB_MG_ES_RJ",
	"RAIS_VINC_PUB_SUL",
	"RAIS_VINC_PUB_NORDESTE",
	"RAIS_VINC_PUB_NORTE",
	"RAIS_VINC_PUB_CENTRO_OESTE",
	"RAIS_VINC_PUB_NI",
}

// RAISCollector downloads annual RAIS data from MTE FTP,
// aggregates by UF+CNAE across all region files, and returns summary records.
type RAISCollector struct {
	ftpHost string
}

// NewRAISCollector creates a new RAISCollector.
// Pass "" for ftpHost to use the default MTE FTP server.
func NewRAISCollector(ftpHost string) *RAISCollector {
	return &RAISCollector{ftpHost: ftpHost}
}

func (c *RAISCollector) Source() string   { return "rais_emprego" }
func (c *RAISCollector) Schedule() string { return "0 3 1 3 *" }

// Collect downloads RAIS region files from MTE FTP for the previous year,
// decodes from Latin-1, and aggregates by (UF, CNAE secao).
func (c *RAISCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	ano := time.Now().Year() - 1

	// We accumulate aggregates across all region files into a single raisAgg map
	// by downloading each file sequentially and feeding rows into aggregateRAISInto.
	merged := make(map[string]*raisAgg)

	for _, region := range raisRegionFiles {
		ftpPath := fmt.Sprintf("%s/RAIS/%d/%s.7z", ftpBasePath, ano, region)

		slog.Info("rais: processing region", "region", region, "year", ano)

		rc, _, err := downloadWithRetry(ctx, c.ftpHost, ftpPath, 3)
		if err != nil {
			slog.Error("rais: skip region", "region", region, "error", err)
			continue // skip failed regions, don't abort entirely
		}

		// RAIS files are Latin-1 encoded -- decode to UTF-8
		decoder := charmap.ISO8859_1.NewDecoder()
		utf8Reader := decoder.Reader(rc)

		if err := aggregateRAISInto(utf8Reader, merged); err != nil {
			slog.Error("rais: aggregate failed", "region", region, "error", err)
		}

		rc.Close()
	}

	return raisAggsToRecords(merged, ano), nil
}
