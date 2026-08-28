package image

import (
	"context"
	"fmt"

	"wood-passage-creator/internal/port"
)

// Picsum 无 Key 的随机图降级（https://picsum.photos）。
type Picsum struct{}

func NewPicsum() *Picsum { return &Picsum{} }

func (p *Picsum) Method() string {
	return MethodPicsum
}
func (p *Picsum) Available() bool {
	return true
}

func (p *Picsum) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	_ = ctx
	seed := req.Position
	if seed <= 0 {
		seed = 1
	}
	return fmt.Sprintf("https://picsum.photos/seed/%d/800/600", seed), nil
}
