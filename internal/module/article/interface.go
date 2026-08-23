package article

import (
	"context"
	"time"

	"wood-passage-creator/internal/pkg/page"
	"wood-passage-creator/internal/port"
)

// Repository 文章持久化端口。
type Repository interface {
	Create(ctx context.Context, params CreateArticleParams) (*Article, error)
	GetByTaskID(ctx context.Context, taskID string) (*Article, error)
	GetByID(ctx context.Context, id int64) (*Article, error)

	Update(ctx context.Context, id int64, params UpdateArticleParams) (*Article, error)
	UpdateByTaskID(ctx context.Context, taskID string, params UpdateArticleParams) (*Article, error)
	UpdateStatus(ctx context.Context, taskID string, status ArticleStatus) error
	UpdatePhase(ctx context.Context, taskID string, phase ArticlePhase) error
	UpdateTitleOptions(ctx context.Context, taskID string, titleOptions []TitleOption) error
	UpdateOutline(ctx context.Context, taskID string, outline []OutlineSection) error

	// ListByUser / ListAll 返回当前页数据与符合条件的总数（供 Service 组装 PageResponse）。
	ListByUser(ctx context.Context, userID int64, params page.PageRequest) (items []*Article, total int, err error)
	ListAll(ctx context.Context, params page.PageRequest) (items []*Article, total int, err error)

	Delete(ctx context.Context, id int64) error
}

// CreateArticleParams 仓储创建参数。
type CreateArticleParams struct {
	UserID              int64
	TaskID              string
	Topic               string
	Style               *ArticleStyle
	EnabledImageMethods []string
}

type UpdateArticleParams struct {
	UserDescription *string
	MainTitle       *string
	SubTitle        *string
	Content         *string
	FullContent     *string
	Status          *ArticleStatus
	Phase           *ArticlePhase
	ErrorMessage    *string
	Style           *ArticleStyle
	CompletedTime   *time.Time

	// JSON / 列表类：nil = 不改；非 nil = 整体覆盖
	TitleOptions        []TitleOption
	Outline             []OutlineSection
	Images              []ImageResult
	EnabledImageMethods []string
}

type AgentOrchestrator interface {
	RunPhase1(ctx context.Context, state *ArticleState) error
	RunPhase2(ctx context.Context, state *ArticleState, streamHandler port.StreamHandler) error
	RunPhase3(ctx context.Context, state *ArticleState, streamHandler port.StreamHandler) error
}
