package image

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"wood-passage-creator/internal/port"
)

type stubProvider struct {
	method string
	url    string
	err    error
}

func (s stubProvider) Method() string { return s.method }
func (s stubProvider) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.url, nil
}

func TestGenerator_PexelsThenOK(t *testing.T) {
	g := &Generator{
		providers: map[string]Provider{
			MethodPexels: stubProvider{method: MethodPexels, url: "https://example.com/a.jpg"},
		},
		fallback:    NewPicsum(),
		concurrency: 2,
	}
	reqs := []port.ImageRequirement{
		{Position: 2, ImageSource: MethodPexels, Keywords: "forest", PlaceholderID: "{{IMAGE_PLACEHOLDER_2}}"},
		{Position: 1, ImageSource: MethodPexels, Keywords: "city", PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}"},
	}
	var prog atomic.Int32
	imgs, err := g.Generate(context.Background(), "t1", reqs, func(ctx context.Context, done, total int, img port.ImageResult) {
		prog.Add(1)
		if total != 2 || done < 1 || done > 2 {
			t.Errorf("progress done=%d total=%d", done, total)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if prog.Load() != 2 {
		t.Fatalf("progress calls=%d", prog.Load())
	}
	if len(imgs) != 2 {
		t.Fatalf("got %d images", len(imgs))
	}
	if imgs[0].Position != 1 || imgs[1].Position != 2 {
		t.Fatalf("not sorted: %+v", imgs)
	}
}

func TestGenerator_FallbackOnError(t *testing.T) {
	g := &Generator{
		providers: map[string]Provider{
			MethodPexels: stubProvider{method: MethodPexels, err: fmt.Errorf("boom")},
		},
		fallback:    NewPicsum(),
		concurrency: 2,
	}
	reqs := []port.ImageRequirement{
		{Position: 1, ImageSource: MethodPexels, PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}"},
	}
	imgs, err := g.Generate(context.Background(), "t1", reqs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 1 || imgs[0].Method != MethodPicsum {
		t.Fatalf("got %+v", imgs)
	}
}

func TestPicsumFetch(t *testing.T) {
	u, err := NewPicsum().Fetch(context.Background(), port.ImageRequirement{Position: 7})
	if err != nil {
		t.Fatal(err)
	}
	if u != "https://picsum.photos/seed/7/800/600" {
		t.Fatal(u)
	}
}

var _ port.ImageGenerator = (*Generator)(nil)
