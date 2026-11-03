package search

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
)

type Searcher interface {
	Search(ctx context.Context, query string, limit int) (*Response, error)
}

type SearchRequest struct {
	Query string `json:"query"`
	Limit *int   `json:"limit"`
}

func RegisterRoutes(mux *http.ServeMux, searcher Searcher) {
	mux.HandleFunc("POST /api/v1/search", SearchHandler(searcher))
}

func SearchHandler(searcher Searcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var req SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		query := strings.TrimSpace(req.Query)
		if query == "" {
			http.Error(w, "query is required", http.StatusBadRequest)
			return
		}

		limit := 5
		if req.Limit != nil {
			limit = *req.Limit
		}
		if limit < 1 || limit > 20 {
			http.Error(w, "limit must be between 1 and 20", http.StatusBadRequest)
			return
		}

		resp, err := searcher.Search(r.Context(), query, limit)
		if err != nil {
			var upstream *UpstreamError

			if errors.As(err, &upstream) &&
				upstream.StatusCode >= 400 &&
				upstream.StatusCode < 500 {
				msg := upstream.Body
				if msg == "" {
					msg = "invalid search request"
				}
				http.Error(w, msg, http.StatusBadRequest)
				return
			}

			log.Printf("search: upstream error: %v", err)
			http.Error(w, "search unavailable", http.StatusBadGateway)
			return
		}

		if resp == nil {
			http.Error(w, "empty response from orchestrator", http.StatusBadGateway)
			return
		}

		if resp.Results == nil {
			resp.Results = []Result{}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "json encoding error", http.StatusBadRequest)
			return
		}
	}
}
