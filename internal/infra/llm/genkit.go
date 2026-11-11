package llm

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai"

	"wood-passage-creator/internal/config"
)

const genkitProvider = "articleai"

// NewGenkit creates the process-wide runtime without contacting the model API.
// Prompt schemas and flows are registered by the application during assembly.
func NewGenkit(ctx context.Context, cfg config.LLMConfig, prompts fs.FS, extraPlugins ...api.Plugin) (*genkit.Genkit, error) {
	if strings.TrimSpace(cfg.Model) == "" || cfg.Model != strings.TrimSpace(cfg.Model) {
		return nil, fmt.Errorf("llm model must be nonempty and contain no surrounding whitespace")
	}
	if cfg.BaseURL != "" {
		u, err := url.Parse(cfg.BaseURL)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
			return nil, fmt.Errorf("llm base URL must be an absolute HTTP or HTTPS URL")
		}
	}
	if prompts == nil {
		return nil, fmt.Errorf("prompt filesystem is required")
	}
	dir, err := fs.Stat(prompts, "prompts")
	if err != nil {
		return nil, fmt.Errorf("open embedded prompts: %w", err)
	}
	if !dir.IsDir() {
		return nil, fmt.Errorf("embedded prompts must be a directory")
	}

	plugin := &compat_oai.OpenAICompatible{
		Provider: genkitProvider,
		APIKey:   cfg.APIKey,
		BaseURL:  cfg.BaseURL,
	}
	g := genkit.Init(ctx,
		genkit.WithPlugins(append([]api.Plugin{plugin}, extraPlugins...)...),
		genkit.WithDefaultModel(genkitProvider+"/"+cfg.Model),
		genkit.WithPromptFS(prompts),
	)
	genkit.RegisterAction(g, plugin.NewModel(cfg.Model, ai.ModelOptions{
		Supports: &ai.ModelSupports{
			Multiturn:   true,
			SystemRole:  true,
			Tools:       true,
			Constrained: ai.ConstrainedSupportAll,
			Output:      []string{ai.OutputFormatText, ai.OutputFormatJSON},
		},
	}))
	return g, nil
}
