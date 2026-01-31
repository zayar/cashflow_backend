package pitixsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"strconv"
)

type pitixClient struct {
	baseURL   string
	apiKey    string
	apiKeyHdr string
	http      *http.Client
	limiter   <-chan time.Time
}

func newPitixClient(apiKey string) (*pitixClient, error) {
	baseURL := strings.TrimSpace(os.Getenv("PITIX_API_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.pitix.com"
	}
	apiKeyHeader := strings.TrimSpace(os.Getenv("PITIX_API_KEY_HEADER"))
	if apiKeyHeader == "" {
		apiKeyHeader = "X-API-Key"
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("pitix api key is empty")
	}
	rateLimitPerMin := int64(10)
	if v := strings.TrimSpace(os.Getenv("PITIX_RATE_LIMIT_PER_MIN")); v != "" {
		if n, err := parseInt64(v); err == nil && n > 0 {
			rateLimitPerMin = n
		}
	}
	interval := time.Minute / time.Duration(rateLimitPerMin)

	return &pitixClient{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKey:    apiKey,
		apiKeyHdr: apiKeyHeader,
		http:      &http.Client{Timeout: 30 * time.Second},
		limiter:   time.Tick(interval),
	}, nil
}

type pitixListResponse struct {
	Data       []json.RawMessage `json:"data"`
	Items      []json.RawMessage `json:"items"`
	NextCursor string            `json:"next_cursor"`
	HasMore    *bool             `json:"has_more"`
}

func (c *pitixClient) getList(ctx context.Context, path string, params url.Values) (pitixListResponse, error) {
	<-c.limiter
	endpoint := c.baseURL + path
	if len(params) > 0 {
		endpoint = endpoint + "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return pitixListResponse{}, err
	}
	req.Header.Set(c.apiKeyHdr, c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return pitixListResponse{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return pitixListResponse{}, fmt.Errorf("pitix api error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed pitixListResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return pitixListResponse{}, err
	}
	return parsed, nil
}

func parseInt64(v string) (int64, error) {
	return strconv.ParseInt(v, 10, 64)
}
