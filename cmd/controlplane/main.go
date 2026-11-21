// Command controlplane runs the whole M1 platform in one process:
// HTTP API, weighted fair scheduler, and worker pool.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/gabbarX/agent-execution-platform/internal/api"
	"github.com/gabbarX/agent-execution-platform/internal/registry"
	"github.com/gabbarX/agent-execution-platform/internal/run"
	"github.com/gabbarX/agent-execution-platform/internal/sched"
	"github.com/gabbarX/agent-execution-platform/internal/worker"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	workers := flag.Int("workers", 4, "worker pool size")
	weightsFlag := flag.String("weights", "", `team weights, e.g. "teamA=2,teamB=1" (default weight 1)`)
	flag.Parse()

	weights, err := parseWeights(*weightsFlag)
	if err != nil {
		log.Fatalf("controlplane: -weights: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	store := run.NewStore()
	reg := registry.New()
	scheduler := sched.New(weights)
	pool := worker.NewPool(*workers, scheduler, store, reg, worker.FakeAgent)
	pool.Start(ctx)

	srv := &http.Server{Addr: *addr, Handler: api.NewServer(store, reg, scheduler, pool)}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("controlplane: shutdown: %v", err)
		}
	}()

	log.Printf("controlplane: listening on %s (workers=%d, weights=%v)", *addr, *workers, weights)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("controlplane: %v", err)
	}
	log.Println("controlplane: stopped")
}

// parseWeights parses "a=2,b=1" into a weights map. Weights must be
// positive integers.
func parseWeights(s string) (map[string]int, error) {
	weights := make(map[string]int)
	if strings.TrimSpace(s) == "" {
		return weights, nil
	}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		team, val, ok := strings.Cut(pair, "=")
		team, val = strings.TrimSpace(team), strings.TrimSpace(val)
		if !ok || team == "" || val == "" {
			return nil, fmt.Errorf("malformed entry %q (want team=weight)", pair)
		}
		w, err := strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("weight for %q: %w", team, err)
		}
		if w < 1 {
			return nil, fmt.Errorf("weight for %q must be >= 1, got %d", team, w)
		}
		weights[team] = w
	}
	return weights, nil
}
