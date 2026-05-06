package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrintTeamTokensDoesNotDumpSecrets(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"print-team-tokens"}, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "api_token") || strings.Contains(got, "hash") {
		t.Fatalf("unexpected secret-like output: %s", got)
	}
	if !strings.Contains(got, "cannot be printed") {
		t.Fatalf("unexpected output: %s", got)
	}
}

func TestCreateTeamPrintsNewToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/admin/teams" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-admin" {
			t.Fatalf("unexpected auth header: %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": 1, "slug": "team-x", "name": "Team X", "api_token": "paa_new"})
	}))
	defer server.Close()

	t.Setenv("ARENA_BASE_URL", server.URL)
	t.Setenv("ARENA_ADMIN_TOKEN", "test-admin")
	var out bytes.Buffer
	if err := run([]string{"create-team", "--slug", "team-x", "--name", "Team X"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "paa_new") {
		t.Fatalf("new token missing from output: %s", out.String())
	}
}

func TestCreateAgentWritesAccessPacketForNewToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-admin" {
			t.Fatalf("unexpected auth header: %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/teams":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{{"id": 1, "slug": "team-x", "name": "Team X", "is_active": true}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/teams/1/agents":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"agent":     map[string]interface{}{"id": 7, "team_id": 1, "team_slug": "team-x", "slug": "default", "name": "Default", "status": "active", "kind": "student"},
				"api_token": "paa_agent_new",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("ARENA_BASE_URL", server.URL)
	t.Setenv("ARENA_ADMIN_TOKEN", "test-admin")
	dir := t.TempDir()
	var out bytes.Buffer
	if err := run([]string{"create-agent", "--team", "team-x", "--slug", "default", "--write-access-packet", "--access-dir", dir}, &out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "team-x-default-access.txt"))
	if err != nil {
		t.Fatal(err)
	}
	packet := string(raw)
	if !strings.Contains(packet, "Token: paa_agent_new") || !strings.Contains(packet, "/api/v1/me") {
		t.Fatalf("unexpected access packet: %s", packet)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
