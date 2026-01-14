package handler

import (
	"encoding/json"
	"net/http"
	"time"
	"url_shortener/internal/store"
)

type URL struct {
	ID          string
	ShortCode   string
	OriginalURL string
	CreatedAt   time.Time
	Clicks      int
}

type ShortenRequest struct {
	URL string `json: "url"`
}

type ShortenResponse struct {
	ShortURL string `json: "short_url"`
}

func ShortenHandler(store store.URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ShortenRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil || req.URL == "" {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		code := generateCode(6)
		store.AddURL(code, req.URL)
		resp := ShortenResponse{
			ShortURL: "http://localhost:8080/" + code,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func RedirectHandler(store store.URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Path[1:]
		url, ok := store.GetURL(code)
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, url, http.StatusFound)

	}
}
