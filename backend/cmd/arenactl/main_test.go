package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
