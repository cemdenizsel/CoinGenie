package httpserver

import (
	"net/http"

	"cg-mentions-bot/internal/handlers"

	"github.com/go-chi/chi/v5"
)

// NewServer creates a simple HTTP server with health and mentions endpoints.
func NewServer(port string, secret string, h handlers.MentionsHandler) *http.Server {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	r.Post("/mentions", h.Handle)

	return &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}
}
