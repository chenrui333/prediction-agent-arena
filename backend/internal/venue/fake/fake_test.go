package fake

import (
	"context"
	"testing"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/venue"
)

func TestPlaceOrderFillsValidOrder(t *testing.T) {
	fake := New()
	result, err := fake.PlaceOrder(context.Background(), venue.PlaceOrderRequest{
		ClientOrderID: "arena-order-9",
		ExternalID:    "bootcamp-demo-1",
		Action:        "buy",
		Outcome:       "yes",
		AmountCents:   10000,
		LimitPriceBPS: 5700,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Filled || result.FillPriceBPS != 5700 {
		t.Fatalf("unexpected fill result: %#v", result)
	}
	if result.VenueOrderID != "arena-order-9" {
		t.Fatalf("venue order id = %q, want deterministic client order id", result.VenueOrderID)
	}
}
