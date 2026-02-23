// Command collector runs scheduled data collectors that sync Brazilian public data
// into the DataBR PostgreSQL database.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/databr/api/internal/collectors/ambiental"
	"github.com/databr/api/internal/collectors/b3"
	"github.com/databr/api/internal/collectors/bcb"
	"github.com/databr/api/internal/collectors/comex"
	"github.com/databr/api/internal/collectors/cvm"
	"github.com/databr/api/internal/collectors/educacao"
	"github.com/databr/api/internal/collectors/emprego"
	"github.com/databr/api/internal/collectors/energia"
	"github.com/databr/api/internal/collectors/ibge"
	"github.com/databr/api/internal/collectors/juridico"
	"github.com/databr/api/internal/collectors/legislativo"
	"github.com/databr/api/internal/collectors/saude"
	"github.com/databr/api/internal/collectors/tesouro"
	"github.com/databr/api/internal/collectors/transporte"
	"github.com/databr/api/internal/collectors/tcu"
	"github.com/databr/api/internal/collectors/transparencia"
	"github.com/databr/api/internal/collectors/tse"
	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/logging"
	"github.com/databr/api/internal/repositories"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

func main() {
	logging.Setup(nil)

	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file, using environment variables")
	}

	// Health endpoint — Railway requires /health to mark the deployment as active.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	healthSrv := &http.Server{Addr: ":" + port, Handler: mux}
	go func() {
		slog.Info("health server started", "port", port)
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Warn("health server error", "error", err)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := repositories.NewPool(ctx)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	repo := repositories.NewSourceRecordRepository(pool)
	runs := repositories.NewCollectorRunRepository(pool)

	// All scheduled collectors (Phases 1–9)
	collectors := []domain.Collector{
		// Phase 1: Core economic data
		bcb.NewSelicCollector(""),
		bcb.NewPTAXCollector(""),
		ibge.NewIPCACollector(""),
		ibge.NewPIBCollector(""),

		// Phase 2: Financial markets
		bcb.NewCreditoCollector(""),
		bcb.NewReservasCollector(""),
		bcb.NewPIXCollector(""),
		cvm.NewFundosCollector(""),
		cvm.NewFatosRelevantesCollector(""),
		b3.NewCotacoesCollector(""),

		// Phase 3: Transparency, treasury & elections
		transparencia.NewPNCPCollector(""),
		tesouro.NewSICONFICollector(""),
		tse.NewCandidatosCollector(""),

		// Phase 4: Expectativas, energy, health, environmental
		bcb.NewFocusCollector(""),
		energia.NewANEELCollector(""),
		saude.NewAnvisaCollector(""),
		ambiental.NewDETERCollector(""),
		ambiental.NewPRODESCollector(""),

		// Phase 5: Transport
		transporte.NewANACCollector(""),
		transporte.NewANTTCollector("", ""),
		transporte.NewPRFCollector(""),

		// Phase 6: Financial rates & market data
		bcb.NewTaxasCreditoCollector(""),
		tesouro.NewTesouroDiretoCollector(""),
		cvm.NewCotasCollector(""),

		// Phase 7: Legislative
		legislativo.NewCamaraCollector(""),
		legislativo.NewSenadoCollector(""),

		// Phase 8: Trade, population, energy, indices
		comex.NewComexStatCollector(""),
		ibge.NewPopulacaoCollector(""),
		energia.NewONSCollector(""),
		b3.NewIndicesCollector(""),

		// Phase 9: Education, employment, judicial, environmental
		educacao.NewINEPCollector(""),
		emprego.NewRAISCollector(""),
		emprego.NewCAGEDCollector(""),
		juridico.NewSTFCollector(""),
		juridico.NewSTJCollector(""),
		ambiental.NewMapBiomasCollector(""),
		ambiental.NewIBAMACollector(""),

		// Phase 12+: TCU
		tcu.NewAcordaosCollector(""),
	}

	// Run all collectors on startup to populate the DB.
	// Heavy collectors (@yearly) run in a separate goroutine to avoid blocking.
	slog.Info("running initial collection for all sources")
	go func() {
		for _, col := range collectors {
			if col.Schedule() == "@yearly" {
				continue
			}
			runCollector(ctx, col, repo, runs)
		}
		slog.Info("initial collection complete (fast sources)")
	}()
	for _, col := range collectors {
		if col.Schedule() != "@yearly" {
			continue
		}
		col := col
		go func() {
			slog.Info("running collector in background (large dataset)", "source", col.Source())
			runCollector(ctx, col, repo, runs)
		}()
	}

	c := cron.New()
	for _, col := range collectors {
		col := col // capture for closure
		if _, err := c.AddFunc(col.Schedule(), func() {
			runCollector(ctx, col, repo, runs)
		}); err != nil {
			slog.Warn("failed to schedule collector", "source", col.Source(), "error", err)
		} else {
			slog.Info("scheduled collector", "source", col.Source(), "schedule", col.Schedule())
		}
	}

	c.Start()
	slog.Info("collector scheduler started")

	<-ctx.Done()
	slog.Info("shutting down collector")

	// Graceful shutdown of health server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := healthSrv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("health server shutdown error", "error", err)
	}

	stopCtx := c.Stop()
	<-stopCtx.Done()
	os.Exit(0)
}

// runCollector executes a single collector and upserts results into the database.
func runCollector(ctx context.Context, col domain.Collector, repo *repositories.SourceRecordRepository, runs *repositories.CollectorRunRepository) {
	slog.Info("collecting", "source", col.Source())
	if runs != nil {
		_ = runs.RecordStart(ctx, col.Source())
	}
	records, err := col.Collect(ctx)
	if err != nil {
		slog.Error("collect failed", "source", col.Source(), "error", err)
		if runs != nil {
			_ = runs.RecordError(ctx, col.Source(), err.Error())
		}
		return
	}
	if len(records) == 0 {
		slog.Info("no records returned", "source", col.Source())
		if runs != nil {
			_ = runs.RecordSuccess(ctx, col.Source(), time.Time{})
		}
		return
	}
	slog.Info("collected records, starting upsert", "source", col.Source(), "count", len(records))
	if err := repo.Upsert(ctx, records); err != nil {
		slog.Error("upsert failed", "source", col.Source(), "error", err)
		if runs != nil {
			_ = runs.RecordError(ctx, col.Source(), err.Error())
		}
		return
	}
	slog.Info("upsert complete", "source", col.Source(), "count", len(records))
	if runs != nil {
		_ = runs.RecordSuccess(ctx, col.Source(), time.Time{})
	}
}
