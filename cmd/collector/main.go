// Command collector runs scheduled data collectors that sync Brazilian public data
// into the DataBR PostgreSQL database.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/databr/api/internal/collectors/ambiental"
	"github.com/databr/api/internal/collectors/b3"
	"github.com/databr/api/internal/collectors/bcb"
	"github.com/databr/api/internal/collectors/cvm"
	"github.com/databr/api/internal/collectors/energia"
	"github.com/databr/api/internal/collectors/ibge"
	"github.com/databr/api/internal/collectors/saude"
	"github.com/databr/api/internal/collectors/tesouro"
	"github.com/databr/api/internal/collectors/transparencia"
	"github.com/databr/api/internal/collectors/tse"
	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/repositories"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file, using environment variables")
	}

	// Health endpoint — Railway requires /health to mark the deployment as active.
	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		log.Printf("[INFO] health server on :%s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Printf("[WARN] health server: %v", err)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := repositories.NewPool(ctx)
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer pool.Close()
	repo := repositories.NewSourceRecordRepository(pool)

	// All scheduled collectors (Phases 1–3)
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
	}

	// Run all collectors on startup to populate the DB.
	// Heavy collectors (@yearly) run in a separate goroutine to avoid blocking.
	log.Println("[INFO] running initial collection for all sources...")
	go func() {
		for _, col := range collectors {
			if col.Schedule() == "@yearly" {
				continue
			}
			runCollector(ctx, col, repo)
		}
		log.Println("[INFO] initial collection complete (fast sources)")
	}()
	for _, col := range collectors {
		if col.Schedule() != "@yearly" {
			continue
		}
		col := col
		go func() {
			log.Printf("[INFO] running %s in background (large dataset)...", col.Source())
			runCollector(ctx, col, repo)
		}()
	}

	c := cron.New()
	for _, col := range collectors {
		col := col // capture for closure
		if _, err := c.AddFunc(col.Schedule(), func() {
			runCollector(ctx, col, repo)
		}); err != nil {
			log.Printf("[WARN] failed to schedule %s: %v", col.Source(), err)
		} else {
			log.Printf("[INFO] scheduled %s at %q", col.Source(), col.Schedule())
		}
	}

	c.Start()
	log.Println("[INFO] collector scheduler started — waiting for schedules")

	<-ctx.Done()
	log.Println("shutting down collector...")
	stopCtx := c.Stop()
	<-stopCtx.Done()
	os.Exit(0)
}

// runCollector executes a single collector and upserts results into the database.
func runCollector(ctx context.Context, col domain.Collector, repo *repositories.SourceRecordRepository) {
	log.Printf("[INFO] collecting %s...", col.Source())
	records, err := col.Collect(ctx)
	if err != nil {
		log.Printf("[ERROR] %s collect: %v", col.Source(), err)
		return
	}
	if len(records) == 0 {
		log.Printf("[INFO] %s: no records returned (weekend/holiday?)", col.Source())
		return
	}
	log.Printf("[INFO] %s: collected %d records, starting upsert...", col.Source(), len(records))
	if err := repo.Upsert(ctx, records); err != nil {
		log.Printf("[ERROR] %s upsert: %v", col.Source(), err)
		return
	}
	log.Printf("[INFO] %s: done — upserted %d records", col.Source(), len(records))
}
