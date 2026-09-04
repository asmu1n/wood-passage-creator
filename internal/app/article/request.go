package article

import (
	module "wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/page"
	"wood-passage-creator/internal/port"
)

// ArticleListData 文章分页 data 形态，供 swag 展示。
type ArticleListData struct {
	Records  []*module.Article `json:"records"`
	Total    int               `json:"total"`
	PageSize int               `json:"pageSize"`
	PageNum  int               `json:"pageNum"`
}

// CreateArticleRequest 创建文章入参（JSON body）。
type CreateArticleRequest struct {
	Topic               string               `json:"topic" validate:"required,min=1,max=512"`
	Style               *module.ArticleStyle `json:"style" validate:"omitempty"`
	EnabledImageMethods []port.ImageMethod   `json:"enabledImageMethods" validate:"omitempty,dive"`
}

// QueryArticleRequest 查询文章入参（query）。
type QueryArticleRequest struct {
	Status *module.ArticleStatus `json:"status" query:"status" validate:"omitempty,oneof=PENDING PROCESSING COMPLETED FAILED"`
	page.PageRequest
}

// ConfirmTitleRequest 确认标题入参。
type ConfirmTitleRequest struct {
	TaskID            string  `json:"taskId" validate:"required,min=1,max=64"`
	SelectedMainTitle string  `json:"selectedMainTitle" validate:"required,min=1,max=512"`
	SelectedSubTitle  string  `json:"selectedSubTitle" validate:"required,min=1,max=512"`
	UserDescription   *string `json:"userDescription" validate:"omitempty,max=4000"`
}

// ConfirmOutlineRequest 确认大纲入参。
type ConfirmOutlineRequest struct {
	TaskID  string                  `json:"taskId" validate:"required,min=1,max=64"`
	Outline []module.OutlineSection `json:"outline" validate:"required,min=1,dive"`
}

// AiModifyOutlineRequest AI 修改大纲入参。
type AiModifyOutlineRequest struct {
	TaskID           string `json:"taskId" validate:"required,min=1,max=64"`
	ModifySuggestion string `json:"modifySuggestion" validate:"required,min=1,max=4000"`
}
