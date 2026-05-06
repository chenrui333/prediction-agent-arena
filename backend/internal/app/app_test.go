package app

import (
	"strings"
	"testing"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/config"
)

func TestNewVenueSelectsFakeByDefault(t *testing.T) {
	got, err := newVenue(config.Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected venue")
	}
}

func TestNewVenueRejectsUnsupportedVenue(t *testing.T) {
	_, err := newVenue(config.Config{Venue: "real-money"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported ARENA_VENUE") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewVenueReportsMissingPolymarketPaperBinary(t *testing.T) {
	_, err := newVenue(config.Config{Venue: "polymarket_paper", PolymarketPaperBin: "definitely-missing-pm-trader"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "polymarket paper binary") {
		t.Fatalf("unexpected error: %v", err)
	}
}
