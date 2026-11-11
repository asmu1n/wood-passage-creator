package articleapp

import (
	"testing"

	"github.com/go-playground/validator/v10"

	module "wood-passage-creator/internal/module/article"
)

func TestCreateValidatesStyleBeforeStartingGeneration(t *testing.T) {
	validate := validator.New()
	for _, style := range []*module.ArticleStyle{
		nil, new(module.ArticleStyle("")), new(module.StyleTech), new(module.StyleEmotional),
		new(module.StyleEducational), new(module.StyleHumorous),
	} {
		if err := validate.Struct(CreateArticleRequest{Topic: "Topic", Style: style}); err != nil {
			t.Errorf("valid style %v rejected: %v", style, err)
		}
	}
	if err := validate.Struct(CreateArticleRequest{Topic: "Topic", Style: new(module.ArticleStyle("unknown"))}); err == nil {
		t.Fatal("unknown style would consume quota before failing in the planning flow")
	}
}
