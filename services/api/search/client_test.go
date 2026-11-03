package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/search" {
			t.Errorf("path = %s, want /search", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %s, want application/json", ct)
		}

		var req searchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req.Query != "где сидела кошка" || req.Limit != 3 {
			t.Errorf("requst = %+v", req)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(Response{
			Results: []Result{
				{
					DocumentID: "doc-1",
					ChunkIndex: 0,
					Content:    "hello",
					Score:      0.9,
				},
			},
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())

	got, err := client.Search(context.Background(), "где сидела кошка", 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	expacted := &Response{
		Results: []Result{
			{
				DocumentID: "doc-1",
				ChunkIndex: 0,
				Content:    "hello",
				Score:      0.9,
			},
		},
	}

	if !reflect.DeepEqual(got, expacted) {
		t.Errorf("got %+v, want %+v", got, expacted)
	}
}
