package polymarket

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every market handed to the reasoning agent must carry an identifier.
//
// Without one the agent has nothing to cite, so it fills the report's
// market_id from whatever text it has. In production that produced a stored
// report keyed on "world-cup-winner moving" — a fragment of the user's
// question. A report keyed like that cannot be grouped by market or scored
// against one, which quietly breaks the accuracy record downstream.
//
// The assertion is made against the marshalled JSON rather than the struct,
// because the JSON is what the model actually sees. A field renamed in its
// tag would still pass a struct-level check while breaking the agent.

func TestContextJSONExposesMarketIdentifiers(t *testing.T) {
	ctx := EventIntelligenceContext{
		Markets: []MarketAnalysis{
			{
				ConditionID: "0xabc123",
				Slug:        "will-spain-win",
				Question:    "Will Spain win the 2026 FIFA World Cup?",
				Probability: 0.999,
			},
		},
	}

	encoded, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshalling context: %v", err)
	}
	payload := string(encoded)

	for _, want := range []string{
		`"condition_id":"0xabc123"`,
		`"slug":"will-spain-win"`,
		`"question":"Will Spain win the 2026 FIFA World Cup?"`,
	} {
		if !strings.Contains(payload, want) {
			t.Errorf("model-facing payload is missing %s\ngot: %s", want, payload)
		}
	}
}

func TestIdentifierFieldsAlwaysSerialise(t *testing.T) {
	// Present even when empty, so the agent can distinguish "no identifier
	// available" from "this payload has no such field" — and so a report is
	// never keyed on invented text.
	encoded, err := json.Marshal(MarketAnalysis{Question: "Q"})
	if err != nil {
		t.Fatalf("marshalling market: %v", err)
	}
	payload := string(encoded)

	if !strings.Contains(payload, `"condition_id"`) {
		t.Errorf("condition_id must always be present: %s", payload)
	}
	if !strings.Contains(payload, `"slug"`) {
		t.Errorf("slug must always be present: %s", payload)
	}
}
