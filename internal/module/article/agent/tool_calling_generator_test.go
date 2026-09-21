package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"wood-passage-creator/internal/port"
)

type stubToolProvider struct {
	method   port.ImageMethod
	url      string
	err      error
	delay    time.Duration
	calls    *atomic.Int32
	inflight *atomic.Int32
	maxIn    *atomic.Int32
}

func (p stubToolProvider) Metadata() port.ImageProviderMetadata {
	metadata := port.ImageProviderMetadata{
		Method: p.method,
		Access: port.ImageAccessFree,
	}
	switch p.method {
	case port.MethodPexels:
		metadata.ToolName = "search_pexels_image"
	case port.MethodMermaid:
		metadata.ToolName = "render_mermaid_diagram"
	}
	return metadata
}

func (p stubToolProvider) Fetch(context.Context, port.ImageRequirement) (string, error) {
	if p.calls != nil {
		p.calls.Add(1)
	}
	if p.inflight != nil && p.maxIn != nil {
		cur := p.inflight.Add(1)
		defer p.inflight.Add(-1)
		for {
			prev := p.maxIn.Load()
			if cur <= prev || p.maxIn.CompareAndSwap(prev, cur) {
				break
			}
		}
	}
	if p.delay > 0 {
		time.Sleep(p.delay)
	}
	if p.err != nil {
		return "", p.err
	}
	return p.url, nil
}

type stubProviderExecutor struct {
	providers map[port.ImageMethod]port.Provider
	log       *slog.Logger
}

func newStubProviderExecutor(providers ...port.Provider) *stubProviderExecutor {
	executor := &stubProviderExecutor{
		providers: make(map[port.ImageMethod]port.Provider, len(providers)),
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	for _, provider := range providers {
		executor.Register(provider)
	}
	return executor
}

func (e *stubProviderExecutor) Register(provider port.Provider) {
	if provider == nil {
		return
	}
	e.providers[provider.Metadata().Method.Normalize()] = provider
}

func (e *stubProviderExecutor) RegisteredMethods() []port.ImageMethod {
	methods := make([]port.ImageMethod, 0, len(e.providers))
	for method := range e.providers {
		methods = append(methods, method)
	}
	sort.Slice(methods, func(i, j int) bool { return methods[i] < methods[j] })
	return methods
}

func (e *stubProviderExecutor) LookupProvider(method port.ImageMethod) (port.ImageProviderMetadata, bool) {
	provider, ok := e.providers[method.Normalize()]
	if !ok {
		return port.ImageProviderMetadata{}, false
	}
	return provider.Metadata(), true
}

func (e *stubProviderExecutor) AvailableProviders(allowedMethods []port.ImageMethod) []port.ImageProviderMetadata {
	providers := make([]port.ImageProviderMetadata, 0, len(e.providers))
	for _, method := range e.RegisteredMethods() {
		if !port.Allow(allowedMethods, method) {
			continue
		}
		metadata, ok := e.LookupProvider(method)
		if ok {
			providers = append(providers, metadata)
		}
	}
	return providers
}

func (e *stubProviderExecutor) Execute(
	ctx context.Context,
	_ string,
	req port.ImageRequirement,
	method port.ImageMethod,
) (port.ImageResult, error) {
	provider, ok := e.providers[method.Normalize()]
	if !ok {
		return port.ImageResult{}, fmt.Errorf("provider not registered: %s", method)
	}
	url, err := provider.Fetch(ctx, req)
	if err != nil {
		return port.ImageResult{}, err
	}
	return port.ImageResult{
		Position:      req.Position,
		URL:           url,
		Method:        method.Normalize(),
		Keywords:      req.Keywords,
		SectionTitle:  req.SectionTitle,
		Description:   req.Type,
		PlaceholderID: req.PlaceholderID,
	}, nil
}

func (e *stubProviderExecutor) ExecuteWithFallback(
	ctx context.Context,
	taskID string,
	req port.ImageRequirement,
) (port.ImageResult, error) {
	return e.Execute(ctx, taskID, req, req.ImageSource)
}

func (e *stubProviderExecutor) Log() *slog.Logger {
	return e.log
}

type stubEinoToolModel struct {
	responses []*schema.Message
	calls     int
	tools     []*schema.ToolInfo
	messages  [][]*schema.Message
}

func (m *stubEinoToolModel) Generate(_ context.Context, messages []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.messages = append(m.messages, append([]*schema.Message(nil), messages...))
	if m.calls >= len(m.responses) {
		return nil, fmt.Errorf("unexpected model call %d", m.calls)
	}
	response := m.responses[m.calls]
	m.calls++
	return response, nil
}

func (m *stubEinoToolModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("stream is not used by image tool calling tests")
}

func (m *stubEinoToolModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	m.tools = append([]*schema.ToolInfo(nil), tools...)
	return m, nil
}

func TestToolCallingGenerator_ExecutesToolInSingleModelCall(t *testing.T) {
	chatModel := &stubEinoToolModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "call-1", Type: "function", Function: schema.FunctionCall{
				Name: "search_pexels_image", Arguments: `{"requirementIndex":0,"keywords":"wood workshop"}`,
			},
		}}),
	}}
	tools := newStubProviderExecutor(
		stubToolProvider{method: port.MethodPexels, url: "https://example.com/wood.jpg"},
	)
	g := NewToolCallingGenerator(chatModel, tools)

	progress := 0
	results, err := g.Generate(context.Background(), "task-1", []port.ImageRequirement{{
		Position: 1, ImageSource: port.MethodPexels, PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}",
	}}, []port.ImageMethod{port.MethodPexels}, func(context.Context, int, int, port.ImageResult) {
		progress++
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Method != port.MethodPexels {
		t.Fatalf("unexpected results: %+v", results)
	}
	if chatModel.calls != 1 {
		t.Fatalf("model calls=%d, want 1", chatModel.calls)
	}
	if progress != 1 {
		t.Fatalf("progress calls=%d, want 1", progress)
	}
	if len(chatModel.tools) != 1 || chatModel.tools[0].Name != "search_pexels_image" {
		t.Fatalf("unexpected tools: %+v", chatModel.tools)
	}
}

func TestToolCallingGenerator_ExecutesMultipleToolsInParallel(t *testing.T) {
	var inflight, maxIn atomic.Int32
	chatModel := &stubEinoToolModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			{
				ID: "call-0", Type: "function", Function: schema.FunctionCall{
					Name: "search_pexels_image", Arguments: `{"requirementIndex":0,"keywords":"a"}`,
				},
			},
			{
				ID: "call-1", Type: "function", Function: schema.FunctionCall{
					Name: "search_pexels_image", Arguments: `{"requirementIndex":1,"keywords":"b"}`,
				},
			},
			{
				ID: "call-2", Type: "function", Function: schema.FunctionCall{
					Name: "search_pexels_image", Arguments: `{"requirementIndex":2,"keywords":"c"}`,
				},
			},
		}),
	}}
	tools := newStubProviderExecutor(
		stubToolProvider{
			method:   port.MethodPexels,
			url:      "https://example.com/img.jpg",
			delay:    80 * time.Millisecond,
			inflight: &inflight,
			maxIn:    &maxIn,
		},
	)
	g := NewToolCallingGenerator(chatModel, tools)

	start := time.Now()
	results, err := g.Generate(context.Background(), "task-parallel", []port.ImageRequirement{
		{Position: 1, ImageSource: port.MethodPexels, PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}"},
		{Position: 2, ImageSource: port.MethodPexels, PlaceholderID: "{{IMAGE_PLACEHOLDER_2}}"},
		{Position: 3, ImageSource: port.MethodPexels, PlaceholderID: "{{IMAGE_PLACEHOLDER_3}}"},
	}, []port.ImageMethod{port.MethodPexels}, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("elapsed %v suggests sequential execution (3x80ms serial ~240ms)", elapsed)
	}
	if maxIn.Load() < 2 {
		t.Fatalf("max concurrent fetches=%d, want >= 2", maxIn.Load())
	}
	if chatModel.calls != 1 {
		t.Fatalf("model calls=%d, want 1", chatModel.calls)
	}
}

func TestToolCallingGenerator_FallsBackAfterToolFailure(t *testing.T) {
	chatModel := &stubEinoToolModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "call-1", Type: "function", Function: schema.FunctionCall{
				Name: "search_pexels_image", Arguments: `{"requirementIndex":0,"keywords":"architecture"}`,
			},
		}}),
	}}
	tools := newStubProviderExecutor(
		stubToolProvider{method: port.MethodPexels, err: fmt.Errorf("no matching photo")},
		stubToolProvider{method: port.MethodMermaid, url: "https://example.com/diagram.svg"},
	)
	g := NewToolCallingGenerator(chatModel, tools)

	results, err := g.Generate(context.Background(), "task-2", []port.ImageRequirement{{
		Position: 1, ImageSource: port.MethodMermaid, PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}",
	}}, []port.ImageMethod{port.MethodPexels, port.MethodMermaid}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Method != port.MethodMermaid {
		t.Fatalf("expected Mermaid default fallback, got %+v", results)
	}
	if chatModel.calls != 1 {
		t.Fatalf("model calls=%d, want 1", chatModel.calls)
	}
}

func TestToolCallingGenerator_SortsResultsWhenToolsFinishOutOfOrder(t *testing.T) {
	chatModel := &stubEinoToolModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			{
				ID: "call-slow", Type: "function", Function: schema.FunctionCall{
					Name: "search_pexels_image", Arguments: `{"requirementIndex":0}`,
				},
			},
			{
				ID: "call-fast", Type: "function", Function: schema.FunctionCall{
					Name: "render_mermaid_diagram", Arguments: `{"requirementIndex":1}`,
				},
			},
		}),
	}}
	tools := newStubProviderExecutor(
		stubToolProvider{method: port.MethodPexels, url: "https://example.com/slow.jpg", delay: 80 * time.Millisecond},
		stubToolProvider{method: port.MethodMermaid, url: "https://example.com/fast.svg"},
	)
	g := NewToolCallingGenerator(chatModel, tools)

	results, err := g.Generate(context.Background(), "task-result-order", []port.ImageRequirement{
		{Position: 1, ImageSource: port.MethodPexels},
		{Position: 2, ImageSource: port.MethodMermaid},
	}, []port.ImageMethod{port.MethodPexels, port.MethodMermaid}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 2 || results[0].Position != 1 || results[1].Position != 2 {
		t.Fatalf("unexpected result order: %+v", results)
	}
}

func TestToolCallingGenerator_FallbackMergesByPosition(t *testing.T) {
	chatModel := &stubEinoToolModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "call-0", Type: "function", Function: schema.FunctionCall{
				Name: "search_pexels_image", Arguments: `{"requirementIndex":0}`,
			},
		}}),
	}}
	tools := newStubProviderExecutor(
		stubToolProvider{method: port.MethodPexels, url: "https://example.com/image.jpg"},
	)
	g := NewToolCallingGenerator(chatModel, tools)

	progress := 0
	results, err := g.Generate(context.Background(), "task-fallback-index", []port.ImageRequirement{
		{Position: 1, ImageSource: port.MethodPexels},
		{Position: 2, ImageSource: port.MethodPexels},
	}, []port.ImageMethod{port.MethodPexels}, func(context.Context, int, int, port.ImageResult) {
		progress++
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Position != 1 || results[1].Position != 2 {
		t.Fatalf("unexpected fallback results: %+v", results)
	}
	if progress != 2 {
		t.Fatalf("progress calls=%d, want 2", progress)
	}
}

func TestToolCallingGenerator_DeduplicatesToolCallsByPosition(t *testing.T) {
	var calls atomic.Int32
	chatModel := &stubEinoToolModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{
			{
				ID: "call-1", Type: "function", Function: schema.FunctionCall{
					Name: "search_pexels_image", Arguments: `{"requirementIndex":0}`,
				},
			},
			{
				ID: "call-2", Type: "function", Function: schema.FunctionCall{
					Name: "search_pexels_image", Arguments: `{"requirementIndex":0}`,
				},
			},
		}),
	}}
	tools := newStubProviderExecutor(
		stubToolProvider{method: port.MethodPexels, url: "https://example.com/image.jpg", calls: &calls},
	)
	g := NewToolCallingGenerator(chatModel, tools)

	progress := 0
	results, err := g.Generate(context.Background(), "task-no-duplicate", []port.ImageRequirement{
		{Position: 1, ImageSource: port.MethodPexels},
	}, []port.ImageMethod{port.MethodPexels}, func(context.Context, int, int, port.ImageResult) {
		progress++
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || calls.Load() != 1 || progress != 1 {
		t.Fatalf("results=%d provider calls=%d progress=%d, want 1/1/1", len(results), calls.Load(), progress)
	}
}

var _ model.ToolCallingChatModel = (*stubEinoToolModel)(nil)
var _ port.ProviderExecutor = (*stubProviderExecutor)(nil)
