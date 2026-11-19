package image

import (
	"context"
	"fmt"
	"testing"

	"wood-passage-creator/internal/port"
)

type stubProvider struct {
	method port.ImageMethod
	access port.ImageProviderAccess
	url    string
	err    error
}

func (s stubProvider) Metadata() port.ImageProviderMetadata {
	access := s.access
	if access == "" {
		access = port.ImageAccessFree
	}
	metadata := port.ImageProviderMetadata{Method: s.method, Access: access}
	switch s.method {
	case port.MethodPexels:
		metadata.ToolName = "search_pexels_image"
	case port.MethodMermaid:
		metadata.ToolName = "render_mermaid_diagram"
	}
	return metadata
}

func (s stubProvider) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.url, nil
}

func TestProviderExecutor_AvailableProviders(t *testing.T) {
	g := &providerExecutor{providers: map[port.ImageMethod]port.Provider{
		port.MethodPexels:  stubProvider{method: port.MethodPexels},
		port.MethodMermaid: stubProvider{method: port.MethodMermaid},
		port.MethodPicsum:  stubProvider{method: port.MethodPicsum, access: port.ImageAccessInternal},
	}}

	providers := g.AvailableProviders([]port.ImageMethod{port.MethodMermaid})
	if len(providers) != 1 {
		t.Fatalf("providers=%d, want 1", len(providers))
	}
	if providers[0].Method != port.MethodMermaid || providers[0].ToolName != "render_mermaid_diagram" {
		t.Fatalf("unexpected provider metadata: %+v", providers[0])
	}
	providers = g.AvailableProviders(nil)
	if len(providers) != 2 {
		t.Fatalf("internal provider leaked into available catalog: %+v", providers)
	}
	if providers := g.AvailableProviders([]port.ImageMethod{}); len(providers) != 0 {
		t.Fatalf("empty allowlist exposed providers: %+v", providers)
	}
}

func TestProviderExecutor_RegisterRejectsIncompleteMetadata(t *testing.T) {
	g := &providerExecutor{}
	g.Register(stubProvider{method: port.MethodPexels, access: "UNKNOWN"})
	if len(g.RegisteredMethods()) != 0 {
		t.Fatalf("provider with invalid access was registered: %v", g.RegisteredMethods())
	}
}

func TestProviderExecutor_LookupProvider(t *testing.T) {
	g := &providerExecutor{providers: map[port.ImageMethod]port.Provider{
		port.MethodPexels: stubProvider{method: port.MethodPexels},
	}}

	metadata, ok := g.LookupProvider(" pexels ")
	if !ok || metadata.Method != port.MethodPexels || metadata.Access != port.ImageAccessFree {
		t.Fatalf("unexpected provider metadata: %+v, ok=%v", metadata, ok)
	}
	if _, ok := g.LookupProvider(port.MethodPicsum); ok {
		t.Fatal("fallback provider must not be exposed through the catalog")
	}
}

func TestProviderExecutor_Execute(t *testing.T) {
	g := &providerExecutor{
		providers: map[port.ImageMethod]port.Provider{
			port.MethodPexels: stubProvider{method: port.MethodPexels, url: "https://example.com/a.jpg"},
		},
	}
	req := port.ImageRequirement{
		Position: 1, Keywords: "city",
	}
	img, err := g.Execute(context.Background(), "t1", req, port.MethodPexels)
	if err != nil {
		t.Fatal(err)
	}
	if img.Position != 1 || img.Method != port.MethodPexels {
		t.Fatalf("unexpected image: %+v", img)
	}
}

func TestProviderExecutor_DoesNotFallbackOnError(t *testing.T) {
	g := &providerExecutor{
		providers: map[port.ImageMethod]port.Provider{
			port.MethodPexels: stubProvider{method: port.MethodPexels, err: fmt.Errorf("boom")},
		},
	}
	req := port.ImageRequirement{
		Position: 1,
	}
	if _, err := g.Execute(context.Background(), "t1", req, port.MethodPexels); err == nil {
		t.Fatal("provider failure must not produce an unrelated fallback image")
	}
}
