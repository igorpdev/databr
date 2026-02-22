//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/databr/api/internal/collectors/bcb"
	"github.com/databr/api/internal/collectors/ibge"
	"github.com/databr/api/internal/domain"
	"github.com/databr/api/internal/repositories"
	"github.com/joho/godotenv"
)

func run(ctx context.Context, repo *repositories.SourceRecordRepository, col domain.Collector) {
	fmt.Printf("Coletando %s...\n", col.Source())
	records, err := col.Collect(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ERRO: %v\n", err)
		return
	}
	if len(records) == 0 {
		fmt.Printf("  Sem dados (fim de semana/feriado?)\n")
		return
	}
	if err := repo.Upsert(ctx, records); err != nil {
		fmt.Fprintf(os.Stderr, "  ERRO upsert: %v\n", err)
		return
	}
	fmt.Printf("  OK — %d registros\n", len(records))
}

func main() {
	godotenv.Load()
	ctx := context.Background()
	pool, err := repositories.NewPool(ctx)
	if err != nil {
		log.Fatal("DB:", err)
	}
	defer pool.Close()
	repo := repositories.NewSourceRecordRepository(pool)

	run(ctx, repo, bcb.NewSelicCollector(""))
	run(ctx, repo, bcb.NewPTAXCollector(""))
	run(ctx, repo, ibge.NewIPCACollector(""))
	run(ctx, repo, ibge.NewPIBCollector(""))
	run(ctx, repo, bcb.NewCreditoCollector(""))
	run(ctx, repo, bcb.NewReservasCollector(""))
}
