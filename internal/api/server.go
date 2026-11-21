// Package api exposes the control plane's HTTP surface.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gabbarX/agent-execution-platform/internal/registry"
	"github.com/gabbarX/agent-execution-platform/internal/run"
	"github.com/gabbarX/agent-execution-platform/internal/sched"
	"github.com/gabbarX/agent-execution-platform/internal/worker"
)

type server struct {
	store *run.Store
	reg   *registry.Registry
	sched *sched.Scheduler
	pool  *worker.Pool
}

// NewServer wires the run API onto a ServeMux.
func NewServer(store *run.Store, reg *registry.Registry, s *sched.Scheduler, pool *worker.Pool) http.Handler {
	srv := &server{store: store, reg: reg, sched: s, pool: pool}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /runs", srv.createRun)
	mux.HandleFunc("GET /runs", srv.listRuns)
	mux.HandleFunc("GET /runs/{id}", srv.getRun)
	mux.HandleFunc("POST /runs/{id}/cancel", srv.cancelRun)
	return mux
}

type createRunRequest struct {
	Team           string `json:"team"`
	Agent          string `json:"agent"`
	Version        int    `json:"version"`
	Input          string `json:"input"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (s *server) createRun(w http.ResponseWriter, r *http.Request) {
	var req createRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Team == "" {
		writeError(w, http.StatusBadRequest, "team is required")
		return
	}
	if req.Agent == "" {
		writeError(w, http.StatusBadRequest, "agent is required")
		return
	}
	if _, ok := s.reg.Lookup(req.Agent, req.Version); !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown agent %s@%d", req.Agent, req.Version))
		return
	}

	created, existed := s.store.Create(&run.Run{
		Team:           req.Team,
		Agent:          req.Agent,
		Version:        req.Version,
		Input:          req.Input,
		IdempotencyKey: req.IdempotencyKey,
	})
	if existed {
		writeJSON(w, http.StatusOK, created)
		return
	}
	s.sched.Enqueue(created.Team, created.ID)
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) listRuns(w http.ResponseWriter, r *http.Request) {
	runs := s.store.List(r.URL.Query().Get("team"))
	if runs == nil {
		runs = []*run.Run{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (s *server) getRun(w http.ResponseWriter, r *http.Request) {
	got, ok := s.store.Get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (s *server) cancelRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cur, ok := s.store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	if cur.Status.Terminal() {
		writeError(w, http.StatusConflict, "run already "+string(cur.Status))
		return
	}

	// Still queued: pull it from the queue and finalize immediately.
	if cur.Status == run.StatusQueued && s.sched.Remove(id) {
		updated, err := s.store.Update(id, func(r *run.Run) error {
			return r.Transition(run.StatusCancelled)
		})
		if err == nil {
			writeJSON(w, http.StatusOK, updated)
			return
		}
		// Lost a race with a worker that dequeued it first — fall
		// through to the executing path.
	}

	// Executing (or about to): ask the pool to interrupt it. The
	// worker finalizes the status, so cancellation is asynchronous.
	s.pool.Cancel(id)
	latest, _ := s.store.Get(id)
	writeJSON(w, http.StatusAccepted, latest)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: encoding response: %v", err)
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
