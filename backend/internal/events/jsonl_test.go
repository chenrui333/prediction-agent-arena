package events

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONLWriteIsValidJSON(t *testing.T) {
	dir := t.TempDir()
	writer := NewWriter(dir)
	if err := writer.Append(context.Background(), "practice-1", "team-01", "heartbeat", map[string]string{"status": "online"}); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(dir, "practice-1", "team-01.events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("expected one line")
	}
	var event Event
	if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event.EventType != "heartbeat" {
		t.Fatalf("unexpected event type: %s", event.EventType)
	}
}
