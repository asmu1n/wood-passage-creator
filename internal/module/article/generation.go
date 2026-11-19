package article

type GenerationWarningCode string

const WarningImageSkipped GenerationWarningCode = "IMAGE_SKIPPED"

// GenerationWarning is a non-fatal result, separate from Article.ErrorMessage.
type GenerationWarning struct {
	Code     GenerationWarningCode `json:"code"`
	ImageID  string                `json:"imageId"`
	Position int                   `json:"position"`
	Message  string                `json:"message"`
}
