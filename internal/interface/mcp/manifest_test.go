package mcp

import (
	"context"
	"sort"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The MCP surface is the only cross-repo seam in PMIE. Cygnus's agents name
// these tools in their prompts, so a rename or a dropped required parameter
// breaks reasoning at runtime, in a Gemini call, with no compile error and no
// failing test anywhere in this repo.
//
// Nothing asserted any of that before: the existing server tests construct a
// Server with nil services and only exercise Host-header handling. This file
// pins the manifest, so changing it becomes a deliberate act with a companion
// change on the Cygnus side.
//
// Amend this when adding a tool. That edit is the point — it is the moment to
// check whether Cygnus needs to learn about it.
var wantTools = map[string][]string{
	"get_event_by_slug":   {"slug"},
	"get_event_by_id":     {"id"},
	"search_markets":      {"query"},
	"get_moving_markets":  nil, // every parameter is optional and defaulted
	"get_whale_activity":  {"slug"},
	"get_market_snapshot": {"slug"},
}

// connectManifestClient registers the tools on a real server and lists them
// over an in-memory transport — the same path a client takes, rather than
// reaching into the registry.
func connectManifestClient(t *testing.T) *mcp.ClientSession {
	t.Helper()

	srv := newTestServer(t)
	srv.RegisterPmiTools()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()

	if _, err := srv.ms.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("connecting server: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "manifest-test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connecting client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	return session
}

func listTools(t *testing.T) map[string]*mcp.Tool {
	t.Helper()

	result, err := connectManifestClient(t).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("listing tools: %v", err)
	}

	byName := make(map[string]*mcp.Tool, len(result.Tools))
	for _, tool := range result.Tools {
		byName[tool.Name] = tool
	}
	return byName
}

// requiredParams reads the "required" list out of a tool's input schema.
// Over the wire the schema arrives as generic JSON (map[string]any), not the
// typed struct the server registered — the SDK documents this asymmetry on
// Tool.InputSchema — so it has to be walked rather than asserted to a type.
func requiredParams(t *testing.T, name string, schema any) []string {
	t.Helper()

	object, ok := schema.(map[string]any)
	if !ok {
		t.Fatalf("%s: input schema is %T, not a JSON object", name, schema)
	}

	raw, present := object["required"]
	if !present {
		return nil
	}

	list, ok := raw.([]any)
	if !ok {
		t.Fatalf("%s: \"required\" is %T, not a list", name, raw)
	}

	fields := make([]string, 0, len(list))
	for _, item := range list {
		field, ok := item.(string)
		if !ok {
			t.Fatalf("%s: required entry %v is %T, not a string", name, item, item)
		}
		fields = append(fields, field)
	}
	return fields
}

func TestManifestExposesExactlyTheExpectedTools(t *testing.T) {
	got := listTools(t)

	var gotNames, wantNames []string
	for name := range got {
		gotNames = append(gotNames, name)
	}
	for name := range wantTools {
		wantNames = append(wantNames, name)
	}
	sort.Strings(gotNames)
	sort.Strings(wantNames)

	if len(gotNames) != len(wantNames) {
		t.Fatalf("tool set changed:\n  got  %v\n  want %v", gotNames, wantNames)
	}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("tool set changed:\n  got  %v\n  want %v", gotNames, wantNames)
		}
	}
}

func TestEveryToolKeepsItsRequiredParameters(t *testing.T) {
	got := listTools(t)

	for name, required := range wantTools {
		tool, ok := got[name]
		if !ok {
			t.Errorf("%s: not registered", name)
			continue
		}
		if tool.InputSchema == nil {
			t.Errorf("%s: no input schema — the client cannot tell what to send", name)
			continue
		}

		actual := requiredParams(t, name, tool.InputSchema)
		have := make(map[string]bool, len(actual))
		for _, field := range actual {
			have[field] = true
		}

		for _, field := range required {
			if !have[field] {
				t.Errorf("%s: %q is no longer required (schema says %v)",
					name, field, actual)
			}
		}
		// A parameter becoming required is just as breaking: Cygnus calls
		// get_moving_markets with no arguments at all.
		if required == nil && len(actual) > 0 {
			t.Errorf("%s: gained required parameters %v; callers pass none",
				name, actual)
		}
	}
}

func TestEveryToolIsDescribedForAnLLM(t *testing.T) {
	// The description is not documentation here — it is what an agent reads to
	// decide whether to call the tool. An empty one makes the tool unusable
	// while every other test still passes.
	for name, tool := range listTools(t) {
		if len(tool.Description) < 20 {
			t.Errorf("%s: description too short to route on: %q", name, tool.Description)
		}
	}
}

func TestServerRegistersNoResourcesOrPrompts(t *testing.T) {
	// Deliberate, and worth pinning so it stays deliberate: resources and
	// prompts are a post-beta refactor. Cygnus is the only client and consumes
	// tools alone, so exposing a half-built resource surface would be a
	// contract nobody reads. Delete this test when that refactor lands.
	session := connectManifestClient(t)
	ctx := context.Background()

	if resources, err := session.ListResources(ctx, nil); err == nil && len(resources.Resources) > 0 {
		t.Errorf("resources registered unexpectedly: %d", len(resources.Resources))
	}
	if prompts, err := session.ListPrompts(ctx, nil); err == nil && len(prompts.Prompts) > 0 {
		t.Errorf("prompts registered unexpectedly: %d", len(prompts.Prompts))
	}
}
