package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	for {
		if err := c.post("/api/v1/heartbeat", map[string]interface{}{
			"status":   "online",
			"metadata": map[string]string{"agent": "momentum-agent-go"},
		}, nil); err != nil {
			fmt.Fprintf(os.Stderr, "heartbeat failed: %v\n", err)
		}
		var markets marketsResponse
		if err := c.get("/api/v1/markets", &markets); err != nil {
			fmt.Fprintf(os.Stderr, "markets failed: %v\n", err)
			time.Sleep(12 * time.Second)
			continue
		}
		for _, m := range markets.Markets {
			outcome, price, estimate, ok := signal(m)
			if !ok {
				continue
			}
			payload := map[string]interface{}{
				"client_order_id":           fmt.Sprintf("momentum-%d", time.Now().UnixNano()),
				"market_id":                 m.ID,
				"outcome":                   outcome,
				"action":                    "buy",
				"amount_cents":              2500,
				"limit_price_bps":           price,
				"estimated_probability_bps": estimate,
				"confidence":                "medium",
				"reason":                    "Price momentum heuristic: follow markets already above 60 percent.",
			}
			if err := c.post("/api/v1/orders", payload, nil); err != nil {
				fmt.Fprintf(os.Stderr, "order rejected: %v\n", err)
			}
			break
		}
		time.Sleep(12 * time.Second)
	}
}

func signal(m market) (string, int64, int64, bool) {
	if m.YesPriceBPS >= 6000 {
		return "yes", m.YesPriceBPS, clamp(m.YesPriceBPS+650, 1, 9999), true
	}
	if m.NoPriceBPS >= 6000 {
		return "no", m.NoPriceBPS, clamp(m.NoPriceBPS+650, 1, 9999), true
	}
	return "", 0, 0, false
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
