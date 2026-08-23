package article

import (
	"time"
	"wood-passage-creator/internal/pkg/page"
)

type Article struct {
	ID                  int64            `json:"id"`
	TaskID              string           `json:"taskId"`
	UserID              int64            `json:"userId"`
	Topic               string           `json:"topic"`
	UserDescription     *string          `json:"userDescription"` // 用户补充描述
	MainTitle           *string          `json:"mainTitle"`
	SubTitle            *string          `json:"subTitle"`
	TitleOptions        []TitleOption    `json:"titleOptions"` // 标题方案列表
	Outline             []OutlineSection `json:"outline"`
	Content             *string          `json:"content"`
	FullContent         *string          `json:"fullContent"`
	Images              []ImageResult    `json:"images"`
	Status              ArticleStatus    `json:"status"`
	Phase               ArticlePhase     `json:"phase"` // 当前阶段
	ErrorMessage        *string          `json:"errorMessage"`
	Style               *ArticleStyle    `json:"style"`               // 文章风格
	EnabledImageMethods []string         `json:"enabledImageMethods"` // 允许的配图方式列表
	CreateTime          time.Time        `json:"createTime"`
	CompletedTime       *time.Time       `json:"completedTime"`
}

type ArticleStatus string

// ArticleStatus 文章状态
const (
	StatusPending    ArticleStatus = "PENDING"
	StatusProcessing ArticleStatus = "PROCESSING"
	StatusCompleted  ArticleStatus = "COMPLETED"
	StatusFailed     ArticleStatus = "FAILED"
)

type ArticlePhase string

// ArticlePhase 文章阶段
const (
	PhasePending           ArticlePhase = "PENDING"            // 等待处理
	PhaseTitleGenerating   ArticlePhase = "TITLE_GENERATING"   // 生成标题中
	PhaseTitleSelecting    ArticlePhase = "TITLE_SELECTING"    // 等待选择标题
	PhaseOutlineGenerating ArticlePhase = "OUTLINE_GENERATING" // 生成大纲中
	PhaseOutlineEditing    ArticlePhase = "OUTLINE_EDITING"    // 等待编辑大纲
	PhaseContentGenerating ArticlePhase = "CONTENT_GENERATING" // 生成正文中
)

type ArticleStyle string

// ArticleStyle 文章风格
const (
	StyleTech        ArticleStyle = "tech"
	StyleEmotional   ArticleStyle = "emotional"
	StyleEducational ArticleStyle = "educational"
	StyleHumorous    ArticleStyle = "humorous"
)

// TitleOption 标题方案
type TitleOption struct {
	MainTitle string `json:"mainTitle"`
	SubTitle  string `json:"subTitle"`
}

// OutlineSection 大纲章节
type OutlineSection struct {
	Section int      `json:"section"`
	Title   string   `json:"title"`
	Points  []string `json:"points"`
}

// ImageRequirement 配图需求
type ImageRequirement struct {
	Position      int    `json:"position"`
	Type          string `json:"type"`
	SectionTitle  string `json:"sectionTitle"`
	ImageSource   string `json:"imageSource"` // PEXELS/NANO_BANANA/MERMAID/ICONIFY/EMOJI_PACK/SVG_DIAGRAM
	Keywords      string `json:"keywords"`
	Prompt        string `json:"prompt"`
	PlaceholderID string `json:"placeholderId"` // {{IMAGE_PLACEHOLDER_N}}
}

// ImageResult 配图结果
type ImageResult struct {
	Position      int    `json:"position"`
	URL           string `json:"url"`
	Method        string `json:"method"`
	Keywords      string `json:"keywords"`
	SectionTitle  string `json:"sectionTitle"`
	Description   string `json:"description"`
	PlaceholderID string `json:"placeholderId"` // {{IMAGE_PLACEHOLDER_N}}
}

// ArticleState 文章生成状态（智能体间共享）
type ArticleState struct {
	TaskID                  string             `json:"taskId"`
	Topic                   string             `json:"topic"`
	UserDescription         string             `json:"userDescription"`     // 用户补充描述
	Style                   *ArticleStyle      `json:"style"`               // 文章风格
	Phase                   ArticlePhase       `json:"phase"`               // 当前阶段
	EnabledImageMethods     []string           `json:"enabledImageMethods"` // 允许的配图方式列表
	TitleOptions            []TitleOption      `json:"titleOptions"`        // 标题方案列表
	MainTitle               *string            `json:"mainTitle"`
	SubTitle                *string            `json:"subTitle"`
	Outline                 []OutlineSection   `json:"outline"`
	Content                 string             `json:"content"`
	ContentWithPlaceholders string             `json:"contentWithPlaceholders"` // 包含占位符的正文
	FullContent             string             `json:"fullContent"`
	ImageRequirements       []ImageRequirement `json:"imageRequirements"`
	Images                  []ImageResult      `json:"images"`
}

// ---------- Swagger 响应辅助类型（仅文档；运行时仍用 page.PageResponse）----------

// ArticleListData 文章分页 data 形态，供 swag 展示（泛型 PageResponse 无法直接被 swag 解析）。
type ArticleListData struct {
	Records  []*Article `json:"records"`
	Total    int        `json:"total"`
	PageSize int        `json:"pageSize"`
	PageNum  int        `json:"pageNum"`
}

// ---------- API 入参（Handler 直接 BindAndValidate 到这些类型）----------
// Echo + go-playground/validator：校验用 validate tag（不是 gin 的 binding）。

// CreateArticleRequest 创建文章入参（JSON body）。
type CreateArticleRequest struct {
	Topic               string        `json:"topic" validate:"required,min=1,max=512"`
	Style               *ArticleStyle `json:"style" validate:"omitempty"`
	EnabledImageMethods []string      `json:"enabledImageMethods" validate:"omitempty,dive,min=1"` // 空/nil 表示支持所有方式
}

// QueryArticleRequest 查询文章入参（query；分页字段来自嵌入的 PageRequest）。
type QueryArticleRequest struct {
	Status *string `json:"status" query:"status" validate:"omitempty,oneof=PENDING PROCESSING COMPLETED FAILED"`
	page.PageRequest
}

// ConfirmTitleRequest 确认标题入参（JSON body）。
type ConfirmTitleRequest struct {
	TaskID            string  `json:"taskId" validate:"required,min=1,max=64"`
	SelectedMainTitle string  `json:"selectedMainTitle" validate:"required,min=1,max=512"`
	SelectedSubTitle  string  `json:"selectedSubTitle" validate:"required,min=1,max=512"`
	UserDescription   *string `json:"userDescription" validate:"omitempty,max=4000"` // 可选
}

// ConfirmOutlineRequest 确认大纲入参（JSON body）。
type ConfirmOutlineRequest struct {
	TaskID  string           `json:"taskId" validate:"required,min=1,max=64"`
	Outline []OutlineSection `json:"outline" validate:"required,min=1,dive"`
}

// AiModifyOutlineRequest AI 修改大纲入参（JSON body）。
type AiModifyOutlineRequest struct {
	TaskID           string `json:"taskId" validate:"required,min=1,max=64"`
	ModifySuggestion string `json:"modifySuggestion" validate:"required,min=1,max=4000"`
}
