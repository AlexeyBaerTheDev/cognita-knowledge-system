package ask

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 150 * time.Second

type Client struct {
	baseURL string
	http    *http.Client
}

type askRequest struct {
	Question string `json:"question"`
	Limit    int    `json:"limit"`
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    httpClient,
	}
}

func (c *Client) Ask(ctx context.Context, question string, limit int) (*Response, error) {
	body, err := json.Marshal(askRequest{
		Question: question,
		Limit:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("create ask request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/ask",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("create ask request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call orchestrator: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, &UpstreamError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(msg)),
		}
	}

	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode ask response: %w", err)
	}

	if out.Sources == nil {
		out.Sources = []Source{}
	}

	return &out, nil
}
