package peer

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"zit/internal/store"
	"zit/internal/tracker"
)

type Server struct {
	addr       string
	localStore *store.LocalStore
	trackerCli *tracker.TrackerClient
	httpServer *http.Server
}

func NewServer(addr string, localStore *store.LocalStore, trackerCli *tracker.TrackerClient) *Server {
	return &Server{
		addr:       addr,
		localStore: localStore,
		trackerCli: trackerCli,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/objects/", s.handleObject)
	mux.HandleFunc("/has/", s.handleHas)
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})
	return mux
}

func (s *Server) handleObject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hash := strings.TrimPrefix(r.URL.Path, "/objects/")
	hash = strings.TrimSpace(hash)
	if hash == "" {
		http.Error(w, "missing hash", http.StatusBadRequest)
		return
	}

	compressed, err := s.localStore.GetCompressed(hash)
	if err != nil {
		http.Error(w, "object not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	w.Write(compressed)
}

func (s *Server) handleHas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hash := strings.TrimPrefix(r.URL.Path, "/has/")
	hash = strings.TrimSpace(hash)
	if hash == "" {
		http.Error(w, "missing hash", http.StatusBadRequest)
		return
	}

	if s.localStore.Has(hash) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("true"))
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("false"))
	}
}

// AnnounceOnce announces all current local objects to the tracker.
func (s *Server) AnnounceOnce() error {
	if s.trackerCli == nil {
		return nil
	}
	hashes, err := s.localStore.List()
	if err != nil {
		return fmt.Errorf("list objects: %w", err)
	}
	if len(hashes) == 0 {
		return nil
	}
	return s.trackerCli.Announce(s.addr, hashes)
}

// Start begins serving HTTP traffic and starts periodic heartbeat announce.
func (s *Server) Start(ctx context.Context) error {
	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: s.Handler(),
	}

	// Initial announce
	if err := s.AnnounceOnce(); err != nil {
		log.Printf("Warning: initial announce failed: %v", err)
	}

	// Heartbeat loop every 1 minute
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.AnnounceOnce(); err != nil {
					log.Printf("Heartbeat announce failed: %v", err)
				}
			}
		}
	}()

	errChan := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}
