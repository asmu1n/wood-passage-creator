package image

import (
	"context"
	"fmt"
	"testing"

	"wood-passage-creator/internal/port"
)

type stubProvider struct {
	method port.ImageMethod
	url    string
	err    error
}

func (s stubProvider) Method() port.ImageMethod { return s.method }
func (s stubProvider) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.url, nil
}

func TestProviderExecutor_Execute(t *testing.T) {
	g := &ProviderExecutor{
		providers: map[port.ImageMethod]port.Provider{
			port.MethodPexels: stubProvider{method: port.MethodPexels, url: "https://example.com/a.jpg"},
		},
		fallback: NewPicsum(),
	}
	req := port.ImageRequirement{
		Position: 1, ImageSource: port.MethodPexels, Keywords: "city", PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}",
	}
	img, err := g.Execute(context.Background(), "t1", req, port.MethodPexels)
	if err != nil {
		t.Fatal(err)
	}
	if img.Position != 1 || img.Method != port.MethodPexels {
		t.Fatalf("unexpected image: %+v", img)
	}
}

func TestProviderExecutor_FallbackOnError(t *testing.T) {
	g := &ProviderExecutor{
		providers: map[port.ImageMethod]port.Provider{
			port.MethodPexels: stubProvider{method: port.MethodPexels, err: fmt.Errorf("boom")},
		},
		fallback: NewPicsum(),
	}
	req := port.ImageRequirement{
		Position: 1, ImageSource: port.MethodPexels, PlaceholderID: "{{IMAGE_PLACEHOLDER_1}}",
	}
	img, err := g.ExecuteWithFallback(context.Background(), "t1", req)
	if err != nil {
		t.Fatal(err)
	}
	if img.Method != port.MethodPicsum {
		t.Fatalf("got %+v", img)
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
