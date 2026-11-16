package repo

import (
	"encoding/json"
	"reflect"
	"testing"

	"wood-passage-creator/ent"
	"wood-passage-creator/internal/module/article"
)

func TestGenerationWarningsPersistAndReload(t *testing.T) {
	warnings := []article.GenerationWarning{{
		Code: article.WarningImageSkipped, ImageID: "image-1", Position: 1, Message: "Cover unavailable",
	}}
	raw, err := json.Marshal(warnings)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := toDomain(&ent.Article{GenerationWarnings: raw})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.GenerationWarnings, warnings) {
		t.Fatalf("warnings lost on reload: %+v", loaded)
	}
	client := ent.NewClient()
	update := client.Article.UpdateOneID(1)
	if err := applyUpdate(update, article.UpdateArticleParams{GenerationWarnings: warnings}); err != nil {
		t.Fatal(err)
	}
	value, ok := update.Mutation().GenerationWarnings()
	if !ok || string(value) != string(raw) {
		t.Fatalf("warning update missing: %+v", value)
	}
	cleared := client.Article.UpdateOneID(1)
	if err := applyUpdate(cleared, article.UpdateArticleParams{GenerationWarnings: []article.GenerationWarning{}}); err != nil {
		t.Fatal(err)
	}
	value, ok = cleared.Mutation().GenerationWarnings()
	if !ok || string(value) != "[]" {
		t.Fatalf("explicit empty warnings did not clear: %+v", value)
	}
	unchanged := client.Article.UpdateOneID(1)
	if err := applyUpdate(unchanged, article.UpdateArticleParams{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := unchanged.Mutation().GenerationWarnings(); ok {
		t.Fatal("nil warnings should leave data unchanged")
	}
	if _, err := toDomain(&ent.Article{GenerationWarnings: []byte(`{"not":"a list"}`)}); err == nil {
		t.Fatal("corrupt warning data must not silently disappear")
	}
}
