package llm_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"wood-passage-creator/internal/app/article/workflow"
	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/infra/llm"
)

type completionRequest struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream"`
	Messages []struct {
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
	ResponseFormat struct {
		Type       string `json:"type"`
		JSONSchema struct {
			Strict bool           `json:"strict"`
			Schema map[string]any `json:"schema"`
		} `json:"json_schema"`
	} `json:"response_format"`
}

func TestGenkitUsesConfiguredEndpointAndStrictSchema(t *testing.T) {
	t.Setenv("GENKIT_REFLECTION_ENABLED", "false")
	t.Setenv("OPENAI_API_KEY", "unrelated-test-key")
	t.Setenv("OPENAI_ORG_ID", "unrelated-test-org")
	t.Setenv("OPENAI_PROJECT_ID", "unrelated-test-project")
	requests := make(chan completionRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected model endpoint: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer configured-test-key" ||
			r.Header.Get("OpenAI-Organization") != "" || r.Header.Get("OpenAI-Project") != "" {
			t.Error("model request used unrelated environment credentials")
		}
		var req completionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- req
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "test-completion", "object": "chat.completion", "model": "configured-model",
			"choices": []any{map[string]any{
				"index": 0, "finish_reason": "stop",
				"message": map[string]any{"role": "assistant", "content": `{"options":[{"mainTitle":"One","subTitle":"First"},{"mainTitle":"Two","subTitle":"Second"},{"mainTitle":"Three","subTitle":"Third"}]}`},
			}},
		})
	}))
	defer server.Close()
	g, err := llm.NewGenkit(t.Context(), config.LLMConfig{
		Model: "configured-model", BaseURL: server.URL + "/v1", APIKey: "configured-test-key",
	}, articleworkflow.PromptFS)
	if err != nil {
		t.Fatal(err)
	}
	planning, err := articleworkflow.NewPlanningFlows(g, nil)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-requests:
		t.Fatal("startup contacted the model API")
	default:
	}
	out, err := planning.GenerateTitles(t.Context(), articleworkflow.GenerateTitlesInput{Topic: "A topic"})
	if err != nil {
		t.Fatal(err)
	}
	req := <-requests
	if len(out.Options) != 3 || req.Model != "configured-model" || req.Stream {
		t.Fatalf("unexpected completion: result=%+v request=%+v", out, req)
	}
	if req.ResponseFormat.Type != "json_schema" || !req.ResponseFormat.JSONSchema.Strict ||
		req.ResponseFormat.JSONSchema.Schema["type"] != "object" ||
		req.ResponseFormat.JSONSchema.Schema["additionalProperties"] != false {
		t.Fatalf("strict structured output was not preserved: %+v", req.ResponseFormat)
	}
}

func TestGenkitStreamsFromOpenAICompatibleEndpoint(t *testing.T) {
	t.Setenv("GENKIT_REFLECTION_ENABLED", "false")
	parts := []string{`{"sections":[{"section":1,"title":"Intro`, `duction","points":["Start here"]}]}`}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req completionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !req.Stream || req.ResponseFormat.Type != "json_schema" {
			t.Errorf("expected a schema-constrained stream: %+v", req)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for i, part := range parts {
			delta := map[string]any{"content": part}
			if i == 0 {
				delta["role"] = "assistant"
			}
			writeCompletionChunk(t, w, delta, nil)
		}
		writeCompletionChunk(t, w, map[string]any{}, "stop")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		w.(http.Flusher).Flush()
	}))
	defer server.Close()
	g, err := llm.NewGenkit(t.Context(), config.LLMConfig{
		Model: "test-model", BaseURL: server.URL + "/v1",
	}, articleworkflow.PromptFS)
	if err != nil {
		t.Fatal(err)
	}
	planning, err := articleworkflow.NewPlanningFlows(g, nil)
	if err != nil {
		t.Fatal(err)
	}
	var snapshots []articleworkflow.OutlineSnapshot
	out, err := planning.GenerateOutline(t.Context(), articleworkflow.GenerateOutlineInput{
		Editorial: articleworkflow.EditorialContext{MainTitle: "Main", SubTitle: "Sub"},
	}, func(_ context.Context, payload articleworkflow.ProgressPayload) error {
		snapshot, ok := payload.(articleworkflow.OutlineSnapshot)
		if !ok {
			t.Fatalf("unexpected outline progress %T", payload)
		}
		snapshots = append(snapshots, snapshot)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 ||
		len(out.Sections) != 1 || out.Sections[0].Title != "Introduction" {
		t.Fatalf("stream was not decoded into snapshots: snapshots=%+v result=%+v", snapshots, out)
	}
}

func writeCompletionChunk(t *testing.T, w http.ResponseWriter, delta map[string]any, finish any) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"id": "test-completion", "object": "chat.completion.chunk", "model": "test-model",
		"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}},
	})
	if err != nil {
		t.Error(err)
		return
	}
	_, _ = w.Write(append(append([]byte("data: "), raw...), '\n', '\n'))
	w.(http.Flusher).Flush()
}

func TestGenkitRejectsInvalidStartupConfiguration(t *testing.T) {
	for _, tt := range []struct {
		name    string
		config  config.LLMConfig
		prompts fs.FS
	}{
		{"missing model", config.LLMConfig{}, articleworkflow.PromptFS},
		{"model whitespace", config.LLMConfig{Model: " model "}, articleworkflow.PromptFS},
		{"relative endpoint", config.LLMConfig{Model: "model", BaseURL: "/v1"}, articleworkflow.PromptFS},
		{"unsupported scheme", config.LLMConfig{Model: "model", BaseURL: "file:///tmp/api"}, articleworkflow.PromptFS},
		{"nil prompts", config.LLMConfig{Model: "model"}, nil},
		{"missing prompt directory", config.LLMConfig{Model: "model"}, fstest.MapFS{}},
		{"prompt path is a file", config.LLMConfig{Model: "model"}, fstest.MapFS{"prompts": &fstest.MapFile{Data: []byte("not a directory")}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := llm.NewGenkit(t.Context(), tt.config, tt.prompts); err == nil {
				t.Fatal("invalid startup configuration accepted")
			}
		})
	}
}
