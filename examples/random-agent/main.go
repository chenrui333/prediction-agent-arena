package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
)

type market struct {
	ID          int64 `json:"id"`
	YesPriceBPS int64 `json:"yes_price_bps"`
	NoPriceBPS  int64 `json:"no_price_bps"`
}

type marketsResponse struct {
	Markets []market `json:"markets"`
}

type client struct {
	baseURL string
	token   string
	http    *http.Client
}

func main() {
	token := strings.TrimSpace(os.Getenv("ARENA_API_TOKEN"))
	if token == "" {
		fmt.Fprintln(os.Stderr, "ARENA_API_TOKEN is required")
		os.Exit(1)
	}
	c := client{
		baseURL: strings.TrimRight(env("ARENA_BASE_URL", "http://localhost:8080"), "/"),
		token:   token,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		if err := c.post("/api/v1/heartbeat", map[string]interface{}{
			"status":   "online",
			"metadata": map[string]string{"agent": "random-agent-go"},
		}, nil); err != nil {
			fmt.Fprintf(os.Stderr, "heartbeat failed: %v\n", err)
		}
		var markets marketsResponse
		if err := c.get("/api/v1/markets", &markets); err != nil {
			fmt.Fprintf(os.Stderr, "markets failed: %v\n", err)
			time.Sleep(8 * time.Second)
			continue
		}
		if len(markets.Markets) > 0 {
			m := markets.Markets[rng.Intn(len(markets.Markets))]
			outcome := []string{"yes", "no"}[rng.Intn(2)]
			price := m.YesPriceBPS
			if outcome == "no" {
				price = m.NoPriceBPS
			}
			estimate := clamp(price+int64(rng.Intn(1401)-700), 1, 9999)
			payload := map[string]interface{}{
				"market_id":                 m.ID,
				"outcome":                   outcome,
				"action":                    "buy",
				"amount_cents":              []int64{1000, 2000, 3000}[rng.Intn(3)],
				"limit_price_bps":           price,
				"estimated_probability_bps": estimate,
				"confidence":                "low",
				"reason":                    "Random low-frequency baseline order.",
			}
			if err := c.post("/api/v1/orders", payload, nil); err != nil {
				fmt.Fprintf(os.Stderr, "order rejected: %v\n", err)
			}
		}
		time.Sleep(time.Duration(8+rng.Intn(7)) * time.Second)
	}
}

func (c client) get(path string, dest interface{}) error {
	return c.do(http.MethodGet, path, nil, dest)
}

func (c client) post(path string, payload interface{}, dest interface{}) error {
	return c.do(http.MethodPost, path, payload, dest)
}

func (c client) do(method, path string, payload interface{}, dest interface{}) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if dest == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

func clamp(value, minValue, maxValue int64) int64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
