package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/httpapi"
	"wood-passage-creator/internal/httpapi/binding"
	httpmw "wood-passage-creator/internal/httpapi/middleware"
	"wood-passage-creator/internal/infra/database"
	"wood-passage-creator/internal/infra/image"
	"wood-passage-creator/internal/infra/llm"
	"wood-passage-creator/internal/infra/redis"
	"wood-passage-creator/internal/module/article"
	articleagent "wood-passage-creator/internal/module/article/agent"
	articlerepo "wood-passage-creator/internal/module/article/repo"
	"wood-passage-creator/internal/module/user"
	userrepo "wood-passage-creator/internal/module/user/repo"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/sse"

	"github.com/labstack/echo-contrib/v5/session"
	"github.com/labstack/echo/v5"
	echomw "github.com/labstack/echo/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "wood-passage-creator/docs/api/swagger"
)

// @title           Go Web API Template
// @version         1.0
// @description     Go Web 后端工程模板接口文档（业务模块由使用者自行接入）
// @host            localhost:8080
// @BasePath        /api
// @securityDefinitions.apikey SessionAuth
// @in header
// @name Cookie
// @description Session cookie authentication. Example: session=your-session-id

func main() {
	cfg := config.LoadConfig()
	logger.Init(&cfg.App)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	e := echo.New()
	e.Validator = binding.NewValidator()
	e.HTTPErrorHandler = httpapi.HTTPErrorHandler

	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(httpmw.AccessLog())

	db, err := database.New(&cfg.Database)
	if err != nil {
		logger.Fatal("connect db failed", logger.FieldErr, err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		logger.Fatal("migrate failed", logger.FieldErr, err)
	}

	store, err := redis.NewSessionStore(&cfg.Redis, cfg.Session)
	if err != nil {
		logger.Fatal("load redis session store failed", logger.FieldErr, err)
	}

	redisClient, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		logger.Fatal("connect redis failed", logger.FieldErr, err)
	}
	defer redisClient.Close()

	chatModal, err := llm.NewChatModel(ctx, cfg.LLM)
	if err != nil {
		logger.Fatal("init chat model failed", logger.FieldErr, err)
	}

	// 业务装配：user 模块（可按需再注入 cache/lock）
	// locker := lock.New(redisClient)
	// cacheClient := cache.New(redisClient)
	// _ = redisClient
	ssehub := sse.NewHub()
	userSvc := user.NewService(userrepo.New(db.Client))
	imgGen := image.NewGenerator(cfg.Pexels)
	articleRepo := articlerepo.NewArticleRepo(db.Client)
	agentLogRepo := articlerepo.NewAgentLogRepo(db.Client)
	agentLogRecorder := article.NewAgentLogRecorder(agentLogRepo)
	articleSvc := article.NewService(
		articleRepo,
		agentLogRepo,
		userSvc,
		articleagent.NewOrchestrator(
			chatModal,
			imgGen,
			articleagent.DefaultImageMethodGuides(),
			agentLogRecorder,
		),
		ssehub,
	)

	e.Use(session.Middleware(store))

	e.GET("/swagger/*", echo.WrapHandler(httpSwagger.WrapHandler))

	httpapi.RegisterRouter(e, userSvc, articleSvc)

	logger.Info("http server starting",
		logger.FieldPurpose, logger.PurposeInfra,
		logger.FieldEvent, "http.listen",
		"addr", ":8080",
	)

	sc := echo.StartConfig{
		Address:    ":8080",
		HideBanner: true,
		BeforeServeFunc: func(s *http.Server) error {
			// SSE 长连接：禁止写超时（0 = 不限制）
			s.WriteTimeout = 0
			return nil
		},
	}
	if err := sc.Start(ctx, e); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal("http server stopped", logger.FieldErr, err)
	}
}
