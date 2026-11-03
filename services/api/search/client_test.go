package search

import (
	"context"
	"encoding/json"
	"errors"
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
			t.Errorf("request = %+v", req)
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

	expected := &Response{
		Results: []Result{
			{
				DocumentID: "doc-1",
				ChunkIndex: 0,
				Content:    "hello",
				Score:      0.9,
			},
		},
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("got %+v, want %+v", got, expected)
	}
}

func TestSearchUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())

	_, err := client.Search(context.Background(), "кошка", 3)
	if err == nil {
		t.Fatal("expected error")
	}

	var upstream *UpstreamError
	if !errors.As(err, &upstream) {
		t.Fatalf("error = %v, want UpstreamError", err)
	}
	if upstream.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", upstream.StatusCode, http.StatusInternalServerError)
	}
	if upstream.Body != "boom" {
		t.Errorf("body = %q, want %q", upstream.Body, "boom")
	}
}

func TestSearchInvalidResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())

	_, err := client.Search(context.Background(), "кошка", 3)
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestSearchEmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())

	resp, err := client.Search(context.Background(), "кошка", 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if resp.Results == nil {
		t.Error("Results is nil, want empty slice")
	}
	if len(resp.Results) != 0 {
		t.Errorf("results = %d, want 0", len(resp.Results))
	}
}
