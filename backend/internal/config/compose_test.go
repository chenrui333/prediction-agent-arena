package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerComposeUsesEnvDrivenLocalBinds(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	compose := string(raw)
	required := []string{
		"env_file:",
		"required: false",
		"${ARENA_BACKEND_BIND:-127.0.0.1}:${ARENA_BACKEND_PORT:-8080}:8080",
		"${ARENA_FRONTEND_BIND:-127.0.0.1}:${ARENA_FRONTEND_PORT:-3000}:3000",
		"${ARENA_REDIS_BIND:-127.0.0.1}:${ARENA_REDIS_PORT:-6379}:6379",
		"ARENA_REDIS_ADDR: \"redis:6379\"",
		"POLYMARKET_PAPER_DATA_DIR: \"/data/pm-trader\"",
	}
	for _, needle := range required {
		if !strings.Contains(compose, needle) {
			t.Fatalf("docker-compose.yml missing %q", needle)
		}
	}

	override, err := os.ReadFile(filepath.Join(root, "docker-compose.exposed.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(override), "0.0.0.0") {
		t.Fatal("exposed override should bind backend/frontend to 0.0.0.0")
	}
	if !strings.Contains(string(override), "127.0.0.1:${ARENA_REDIS_PORT:-6379}:6379") {
		t.Fatal("exposed override must keep Redis local-only")
	}
}
