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
	ListByUser(ctx context.Context, userID int64, params ListArticlesParams) (items []*Article, total int, err error)
	List(ctx context.Context, params ListArticlesParams) (items []*Article, total int, err error)

	Delete(ctx context.Context, id int64) error // repo only
}

// CreateArticleParams 仓储创建参数。
type CreateArticleParams struct {
	UserID              int64
	TaskID              string
	Topic               string
	Style               *ArticleStyle
	EnabledImageMethods []port.ImageMethod
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
	Images              []port.ImageResult
	EnabledImageMethods []port.ImageMethod
}

// ListArticlesParams 仓储列表过滤（无 HTTP tag）。
type ListArticlesParams struct {
	Status *ArticleStatus
	page.PageRequest
}

// ProgressFunc 任务进度回调（由 Service 注入，内部通常 publish 到 SSE）。
// name 为 SSE event 名；data 为 payload（将被 JSON 序列化）。
type ProgressFunc func(ctx context.Context, name string, data any)

type AgentOrchestrator interface {
	RunPhase1(ctx context.Context, state *ArticleState) error
	RunPhase2(ctx context.Context, state *ArticleState, onProgress ProgressFunc) error
	RunPhase3(ctx context.Context, state *ArticleState, onProgress ProgressFunc) error
	ModifyOutline(ctx context.Context, state *ArticleState, modifySuggestion string) ([]OutlineSection, error)
}

// CreateAgentLogParams 写入一条执行日志。
type CreateAgentLogParams struct {
	ArticleID    int64
	TaskID       string
	AgentName    string
	StartTime    time.Time
	EndTime      time.Time
	DurationMs   int
	Status       AgentLogStatus
	ErrorMessage string
	InputData    string
	OutputData   string
	// Prompt 默认不写或截断，由调用方决定是否传入
	Prompt string
}

// AgentLogRepository 执行日志持久化。
type AgentLogRepository interface {
	Create(ctx context.Context, params CreateAgentLogParams) error
	ListByTaskID(ctx context.Context, taskID string) ([]*AgentLog, error)
}

// AgentLogRecorder 编排层打点用；实现应异步落库且不因失败影响主流程。
// 可为 nil：编排层判空后跳过，不提供空实现。
type AgentLogRecorder interface {
	SaveAsync(params CreateAgentLogParams)
}
