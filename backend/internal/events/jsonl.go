package events

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Writer struct {
	dir string
	mu  sync.Mutex
}

type Event struct {
	TS        string      `json:"ts"`
	RoundSlug string      `json:"round_slug"`
	TeamSlug  string      `json:"team_slug"`
	EventType string      `json:"event_type"`
	Payload   interface{} `json:"payload"`
}

func NewWriter(dir string) *Writer {
	return &Writer{dir: dir}
}

func (w *Writer) Append(ctx context.Context, roundSlug, teamSlug, eventType string, payload interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if teamSlug == "" {
		teamSlug = "admin"
	}
	event := Event{
		TS:        time.Now().UTC().Format(time.RFC3339Nano),
		RoundSlug: clean(roundSlug),
		TeamSlug:  clean(teamSlug),
		EventType: eventType,
		Payload:   payload,
	}
	line, err := json.Marshal(event)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	path := filepath.Join(w.dir, event.RoundSlug, event.TeamSlug+".events.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	buf := bufio.NewWriter(file)
	if _, err := buf.Write(line); err != nil {
		return err
	}
	if err := buf.WriteByte('\n'); err != nil {
		return err
	}
	return buf.Flush()
}

func clean(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, "..", "-")
	return value
}
