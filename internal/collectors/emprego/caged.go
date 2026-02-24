package emprego

import (
	"context"
	"fmt"
	"time"

	"github.com/databr/api/internal/domain"
)

// CAGEDCollector downloads monthly CAGED data from MTE FTP,
// aggregates by UF+CNAE in memory, and returns summary records.
type CAGEDCollector struct {
	ftpHost string // override for testing; "" uses default
}

// NewCAGEDCollector creates a new CAGEDCollector.
// Pass "" for ftpHost to use the default MTE FTP server.
func NewCAGEDCollector(ftpHost string) *CAGEDCollector {
	return &CAGEDCollector{ftpHost: ftpHost}
}

func (c *CAGEDCollector) Source() string   { return "caged_emprego" }
func (c *CAGEDCollector) Schedule() string { return "0 12 1 * *" }

// Collect downloads the previous month's CAGED data from MTE FTP,
// extracts the 7z archive, and aggregates by (UF, CNAE secao).
func (c *CAGEDCollector) Collect(ctx context.Context) ([]domain.SourceRecord, error) {
	prevMonth := time.Now().AddDate(0, -1, 0)
	periodo := prevMonth.Format("200601")
	ftpPath := fmt.Sprintf("%s/NOVO CAGED/%d/%s/CAGEDMOV%s.7z",
		ftpBasePath, prevMonth.Year(), periodo, periodo)

	rc, entryName, err := downloadWithRetry(ctx, c.ftpHost, ftpPath, 3)
	if err != nil {
		return nil, fmt.Errorf("caged_emprego: %w", err)
	}
	defer rc.Close()

	_ = entryName // logged inside downloadAndExtract7z
	return aggregateCAGED(rc, periodo)
}
