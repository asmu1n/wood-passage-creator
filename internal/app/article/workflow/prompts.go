package articleworkflow

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"google.golang.org/genai"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/port"
)

const (
	PromptTitles        = "titles"
	PromptOutline       = "outline"
	PromptReviseOutline = "revise-outline"
	PromptContent       = "content"
	PromptImagePlan     = "image-plan"
	PromptImageExecute  = "image-execute"
	PromptMermaid       = "mermaid"
	PromptSVG           = "svg"
)

// PromptFS embeds the approved templates and underscore-prefixed partials.
//
//go:embed all:prompts
var PromptFS embed.FS

// Prompt inputs deliberately exclude RunContext and execution budgets.
type TitlesPromptInput struct {
	Topic string `json:"topic"`
	Style string `json:"style,omitempty"`
}

// Handlebars helpers compare scalar strings, not pointers or Go enum aliases.
type PromptEditorialContext struct {
	MainTitle       string `json:"mainTitle"`
	SubTitle        string `json:"subTitle"`
	Style           string `json:"style,omitempty"`
	UserDescription string `json:"userDescription"`
}

type OutlinePromptInput struct {
	Editorial PromptEditorialContext `json:"editorial"`
}

type ReviseOutlinePromptInput struct {
	Editorial        PromptEditorialContext   `json:"editorial"`
	Sections         []article.OutlineSection `json:"sections"`
	ModifySuggestion string                   `json:"modifySuggestion"`
}

type ContentPromptInput struct {
	Editorial PromptEditorialContext   `json:"editorial"`
	Outline   []article.OutlineSection `json:"outline"`
}

type ImageProviderGuide struct {
	Method      port.ImageMethod `json:"method"`
	Description string           `json:"description"`
	UsageGuide  string           `json:"usageGuide"`
}

type PlanImagesInput struct {
	Editorial EditorialContext     `json:"editorial"`
	Document  ArticleDocument      `json:"document"`
	Providers []ImageProviderGuide `json:"providers"`
}

// ToolName and UsageGuide come from the same authorized Provider Catalog.
type ImageToolGuide struct {
	Method     port.ImageMethod `json:"method"`
	ToolName   string           `json:"toolName"`
	UsageGuide string           `json:"usageGuide"`
}

type ImageExecutePromptInput struct {
	Kind            ImageKind        `json:"kind"`
	Brief           string           `json:"brief"`
	PreferredMethod port.ImageMethod `json:"preferredMethod"`
	Tools           []ImageToolGuide `json:"tools"`
}

type DiagramPromptInput struct {
	Description string `json:"description"`
}

func registerPromptSchemas(g *genkit.Genkit) {
	genkit.DefineSchemasFor(g,
		TitlesPromptInput{}, OutlinePromptInput{}, ReviseOutlinePromptInput{}, ContentPromptInput{},
		PlanImagesInput{}, ImageExecutePromptInput{}, DiagramPromptInput{},
		TitlesResult{}, OutlineResult{}, ImagePlan{},
	)
}

func promptStyle(style *article.ArticleStyle) string {
	if style == nil {
		return ""
	}
	return string(*style)
}

func promptEditorial(input EditorialContext) PromptEditorialContext {
	return PromptEditorialContext{
		MainTitle: input.MainTitle, SubTitle: input.SubTitle,
		Style: promptStyle(input.Style), UserDescription: input.UserDescription,
	}
}

// NewImageSources binds approved source prompts to the same runtime as the
// article flow. NewPlanningFlows must register prompt schemas first.
func NewImageSources(g *genkit.Genkit, nano config.NanoBananaConfig) (port.ImageSources, error) {
	if g == nil {
		return port.ImageSources{}, fmt.Errorf("Genkit runtime is required")
	}
	mermaid := genkit.LookupDataPrompt[DiagramPromptInput, string](g, PromptMermaid)
	svg := genkit.LookupDataPrompt[DiagramPromptInput, string](g, PromptSVG)
	if mermaid == nil || svg == nil {
		return port.ImageSources{}, fmt.Errorf("diagram prompts are missing")
	}
	bind := func(prompt *ai.DataPrompt[DiagramPromptInput, string]) func(context.Context, string) (string, error) {
		return func(ctx context.Context, description string) (string, error) {
			source, resp, err := prompt.Execute(ctx, DiagramPromptInput{Description: description})
			if err != nil {
				return "", err
			}
			if err := validateCompletedResponse(resp); err != nil {
				return "", err
			}
			if strings.TrimSpace(source) == "" {
				return "", fmt.Errorf("diagram generation returned empty source")
			}
			return source, nil
		}
	}
	sources := port.ImageSources{Mermaid: bind(mermaid), SVG: bind(svg)}
	if strings.TrimSpace(nano.APIKey) != "" {
		nano = nano.Normalized()
		sources.NanoBanana = func(ctx context.Context, description string) (string, error) {
			resp, err := genkit.Generate(ctx, g, ai.WithModelName("googleai/"+nano.Model), ai.WithPrompt(description),
				ai.WithConfig(&genai.GenerateContentConfig{
					ResponseModalities: []string{"TEXT", "IMAGE"},
					ImageConfig:        &genai.ImageConfig{AspectRatio: nano.AspectRatio},
				}))
			if err != nil {
				return "", err
			}
			if err := validateCompletedResponse(resp); err != nil {
				return "", err
			}
			if resp.Message != nil {
				for _, part := range resp.Message.Content {
					if part.IsImage() && strings.HasPrefix(part.Text, "data:image/") {
						return part.Text, nil
					}
				}
			}
			return "", fmt.Errorf("AI image model returned no image")
		}
	}
	return sources, nil
}
