package port

import "context"

// ImageSources inject model-backed capabilities without exposing an AI SDK to
// the provider catalog. Nil functions leave the corresponding provider disabled.
type ImageSources struct {
	Mermaid    func(context.Context, string) (string, error)
	SVG        func(context.Context, string) (string, error)
	NanoBanana func(context.Context, string) (string, error)
}
