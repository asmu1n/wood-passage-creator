package image

import (
	"context"
	"fmt"
	"testing"

	"wood-passage-creator/internal/port"
)

type stubProvider struct {
	method string
	url    string
	err    error
	avail  bool
}

func (s stubProvider) Method() string  { return s.method }
func (s stubProvider) Available() bool { return s.avail }
func (s stubProvider) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.url, nil
}

func TestGenerator_PexelsThenOK(t *testing.T) {
	g := &Generator{
		providers: map[string]Provider{
			MethodPexels: stubProvider{method: MethodPexels, url: "https://example.com/a.jpg", avail: true},
		},
		fallback:    NewPicsum(),
		concurrency: 2,
	}
	reqs := []port.ImageRequirement{
		{Position: 2, ImageSource: MethodPexels, Keywords: "forest", PlaceholderID: "{{IMAGE_PLACEHOLDER_2}}"},
		{Position: 1, ImageSource: MethodPexels, Keywords: "city", PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}"},
	}
	imgs, err := g.Generate(context.Background(), "t1", reqs)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 2 {
		t.Fatalf("got %d images", len(imgs))
	}
	if imgs[0].Position != 1 || imgs[1].Position != 2 {
		t.Fatalf("not sorted: %+v", imgs)
	}
	if imgs[0].PlaceholderID != "{{IMAGE_PLACEHOLDER_1}}" {
		t.Fatalf("placeholder: %s", imgs[0].PlaceholderID)
	}
}

func TestGenerator_FallbackOnError(t *testing.T) {
	g := &Generator{
		providers: map[string]Provider{
			MethodPexels: stubProvider{method: MethodPexels, avail: true, err: fmt.Errorf("boom")},
		},
		fallback:    NewPicsum(),
		concurrency: 2,
	}
	reqs := []port.ImageRequirement{
		{Position: 1, ImageSource: MethodPexels, PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}"},
	}
	imgs, err := g.Generate(context.Background(), "t1", reqs)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 1 {
		t.Fatalf("want 1 image via fallback, got %d", len(imgs))
	}
	if imgs[0].Method != MethodPicsum {
		t.Fatalf("method=%s", imgs[0].Method)
	}
	if imgs[0].URL == "" {
		t.Fatal("empty url")
	}
}

func TestPicsumFetch(t *testing.T) {
	u, err := NewPicsum().Fetch(context.Background(), port.ImageRequirement{Position: 7})
	if err != nil {
		t.Fatal(err)
	}
	want := "https://picsum.photos/seed/7/800/600"
	if u != want {
		t.Fatalf("got %s want %s", u, want)
	}
}

// 编译期断言：Generator 实现 port.ImageGenerator
var _ port.ImageGenerator = (*Generator)(nil)
