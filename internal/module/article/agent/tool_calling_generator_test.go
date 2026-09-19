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

func TestToolCallingGenerator_ExecutesToolAndReturnsObservation(t *testing.T) {
	chatModel := &stubEinoToolModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "call-1", Type: "function", Function: schema.FunctionCall{
				Name: "search_pexels_image", Arguments: `{"requirementIndex":0,"keywords":"wood workshop"}`,
			},
		}}),
		schema.AssistantMessage("全部图片已生成。", nil),
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
	if chatModel.calls != 2 {
		t.Fatalf("model calls=%d, want 2", chatModel.calls)
	}
	if progress != 1 {
		t.Fatalf("progress calls=%d, want 1", progress)
	}
	last := chatModel.messages[1][len(chatModel.messages[1])-1]
	if last.Role != schema.Tool || last.ToolCallID != "call-1" {
		t.Fatalf("missing tool observation: %+v", last)
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
		schema.AssistantMessage("全部图片已生成。", nil),
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
	msgs := chatModel.messages[1]
	toolMsgs := make([]*schema.Message, 0, 3)
	for _, m := range msgs {
		if m.Role == schema.Tool {
			toolMsgs = append(toolMsgs, m)
		}
	}
	if len(toolMsgs) != 3 {
		t.Fatalf("tool messages=%d, want 3", len(toolMsgs))
	}
	for i, wantID := range []string{"call-0", "call-1", "call-2"} {
		if toolMsgs[i].ToolCallID != wantID {
			t.Fatalf("tool message %d id=%s, want %s", i, toolMsgs[i].ToolCallID, wantID)
		}
	}
}

func TestToolCallingGenerator_ChangesToolAfterFailure(t *testing.T) {
	chatModel := &stubEinoToolModel{responses: []*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "call-1", Type: "function", Function: schema.FunctionCall{
				Name: "search_pexels_image", Arguments: `{"requirementIndex":0,"keywords":"architecture"}`,
			},
		}}),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID: "call-2", Type: "function", Function: schema.FunctionCall{
				Name: "render_mermaid_diagram", Arguments: `{"requirementIndex":0,"prompt":"flowchart LR; A-->B"}`,
			},
		}}),
		schema.AssistantMessage("完成。", nil),
	}}
	tools := newStubProviderExecutor(
		stubToolProvider{method: port.MethodPexels, err: fmt.Errorf("no matching photo")},
		stubToolProvider{method: port.MethodMermaid, url: "https://example.com/diagram.svg"},
	)
	g := NewToolCallingGenerator(chatModel, tools)

	results, err := g.Generate(context.Background(), "task-2", []port.ImageRequirement{{
		Position: 1, ImageSource: port.MethodPexels, PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}",
	}}, []port.ImageMethod{port.MethodPexels, port.MethodMermaid}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Method != port.MethodMermaid {
		t.Fatalf("expected Mermaid fallback selected by model, got %+v", results)
	}
	if chatModel.calls != 3 {
		t.Fatalf("model calls=%d, want 3", chatModel.calls)
	}
}

var _ model.ToolCallingChatModel = (*stubEinoToolModel)(nil)
var _ port.ProviderExecutor = (*stubProviderExecutor)(nil)
