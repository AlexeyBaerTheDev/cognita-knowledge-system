package ask

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
)

type Asker interface {
	Ask(ctx context.Context, question string, limit int) (*Response, error)
}

type AskRequest struct {
	Question string `json:"question"`
	Limit    *int   `json:"limit"`
}

func RegisterRoutes(mux *http.ServeMux, asker Asker) {
	mux.HandleFunc("POST /api/v1/ask", AskHandler(asker))
}

func AskHandler(asker Asker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var req AskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		question := strings.TrimSpace(req.Question)
		if question == "" {
			http.Error(w, "question is required", http.StatusBadRequest)
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

		resp, err := asker.Ask(r.Context(), question, limit)
		if err != nil {
			var upstream *UpstreamError

			if errors.As(err, &upstream) &&
				upstream.StatusCode >= 400 &&
				upstream.StatusCode < 500 {
				msg := upstream.Body
				if msg == "" {
					msg = "invalid ask request"
				}
				http.Error(w, msg, http.StatusBadRequest)
				return
			}

			log.Printf("ask: upstream error: %v", err)
			http.Error(w, "ask unavailable", http.StatusBadGateway)
			return
		}

		if resp == nil {
			http.Error(w, "empty response from orchestrator", http.StatusBadGateway)
			return
		}

		if resp.Sources == nil {
			resp.Sources = []Source{}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("ask: encode response: %v", err)
		}
	}
}
