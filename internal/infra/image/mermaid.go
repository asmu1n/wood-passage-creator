package image

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/port"
)

type Mermaid struct {
	cfg config.MermaidConfig
}

func NewMermaid(cfg config.MermaidConfig) *Mermaid {
	cfg = cfg.Normalized()
	if cfg.CLI == "" {
		return nil
	}
	if _, err := exec.LookPath(cfg.CLI); err != nil {
		return nil
	}
	return &Mermaid{cfg: cfg}
}

func (p *Mermaid) Method() port.ImageMethod { return port.MethodMermaid }

func (p *Mermaid) Fetch(ctx context.Context, req port.ImageRequirement) (string, error) {
	code := reqText(req, true)
	if strings.TrimSpace(code) == "" {
		return "", fmt.Errorf("mermaid: prompt/code required")
	}

	in, err := os.CreateTemp("", "mermaid_in_*.mmd")
	if err != nil {
		return "", err
	}
	inPath := in.Name()
	defer os.Remove(inPath)
	if _, err := in.WriteString(code); err != nil {
		in.Close()
		return "", err
	}
	in.Close()

	ext := "." + strings.TrimPrefix(p.cfg.OutputFormat, ".")
	out, err := os.CreateTemp("", "mermaid_out_*"+ext)
	if err != nil {
		return "", err
	}
	outPath := out.Name()
	out.Close()
	defer os.Remove(outPath)

	timeout := time.Duration(p.cfg.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"-i", inPath,
		"-o", outPath,
		"-t", p.cfg.Theme,
		"-w", fmt.Sprintf("%d", p.cfg.Width),
		"-H", fmt.Sprintf("%d", p.cfg.Height),
	}
	cmd := exec.CommandContext(cctx, p.cfg.CLI, args...)
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("mermaid cli: %w: %s", err, truncate(string(b), 300))
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		return "", err
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("mermaid: empty output")
	}
	mime := "image/png"
	switch strings.ToLower(p.cfg.OutputFormat) {
	case "svg":
		mime = "image/svg+xml"
	case "pdf":
		mime = "application/pdf"
	}
	return dataURL(mime, raw), nil
}
