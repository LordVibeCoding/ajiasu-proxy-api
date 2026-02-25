package api

import (
	"net/http"
	"strings"

	"ajiasu-proxy-api/internal/ajiasu"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(token string, mgr *ajiasu.Manager) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	h := NewAjiasuHandler(mgr)

	r.Route("/api/ajiasu", func(r chi.Router) {
		r.Use(tokenAuth(token))
		r.Get("/status", h.Status)
		r.Get("/nodes", h.ListNodes)
		r.Post("/connect", h.Connect)
		r.Post("/disconnect", h.Disconnect)
		r.Post("/auto", h.AutoSelect)
	})

	return r
}

func tokenAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}
			auth := r.Header.Get("Authorization")
			if auth == "" {
				auth = r.URL.Query().Get("token")
			} else {
				auth = strings.TrimPrefix(auth, "Bearer ")
			}
			if auth != token {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
