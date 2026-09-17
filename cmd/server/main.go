package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	appcore "wood-passage-creator/internal/app"
	articleapp "wood-passage-creator/internal/app/article"
	authapp "wood-passage-creator/internal/app/auth"
	paymentapp "wood-passage-creator/internal/app/payment"
	statsapp "wood-passage-creator/internal/app/statistics"
	userapp "wood-passage-creator/internal/app/user"
	"wood-passage-creator/internal/config"
	"wood-passage-creator/internal/httpapi"
	articleapi "wood-passage-creator/internal/httpapi/api/article"
	authapi "wood-passage-creator/internal/httpapi/api/auth"
	paymentapi "wood-passage-creator/internal/httpapi/api/payment"
	statisticsapi "wood-passage-creator/internal/httpapi/api/statistics"
	userapi "wood-passage-creator/internal/httpapi/api/user"
	"wood-passage-creator/internal/httpapi/binding"
	"wood-passage-creator/internal/httpapi/docsui"
	httpmw "wood-passage-creator/internal/httpapi/middleware"
	"wood-passage-creator/internal/infra/cache"
	"wood-passage-creator/internal/infra/database"
	"wood-passage-creator/internal/infra/image"
	"wood-passage-creator/internal/infra/llm"
	"wood-passage-creator/internal/infra/objectstore"
	"wood-passage-creator/internal/infra/redis"
	modart "wood-passage-creator/internal/module/article"
	articleagent "wood-passage-creator/internal/module/article/agent"
	articlerepo "wood-passage-creator/internal/module/article/repo"
	paymentrepo "wood-passage-creator/internal/module/payment/repo"
	statisticsrepo "wood-passage-creator/internal/module/statistics/repo"
	userrepo "wood-passage-creator/internal/module/user/repo"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/sse"

	_ "wood-passage-creator/docs/api/swagger"

	"github.com/labstack/echo-contrib/v5/session"
	"github.com/labstack/echo/v5"
	echomw "github.com/labstack/echo/v5/middleware"
)

// @title           Wood Passage Creator API
// @version         1.0
// @description     AI 文章生成后端：用户/会话、文章三阶段流水线（SSE 进度）、配图、Agent 日志；开发态 VIP Mock 支付与管理端升降 VIP。
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
	if err := appcore.InitRuntime(appcore.Runtime{TxManager: db}); err != nil {
		logger.Fatal("init app runtime failed", logger.FieldErr, err)
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

	einoModel, err := llm.NewChatModel(ctx, cfg.LLM)
	if err != nil {
		logger.Fatal("init chat model failed", logger.FieldErr, err)
	}

	// 装配：module=领域+repo；app=全部用例；httpapi/api 只依赖 app。
	// 跨 module 强一致写：Runtime.Tx + repo Cli(ctx) 共享同一事务。
	cacheClient := cache.New(redisClient)
	ssehub := sse.NewHub()
	userRepo := userrepo.New(db)
	statsSvc := statsapp.NewService(statisticsrepo.New(db), cacheClient)
	userSvc := userapp.NewService(userRepo, statsSvc)
	authSvc := authapp.NewService(userRepo, statsSvc)
	paymentSvc := paymentapp.NewService(paymentrepo.New(db), userSvc)
	objStore := objectstore.NewFromConfig(cfg.R2)
	imgGen := image.NewToolCallingGenerator(cfg, einoModel, objStore)
	articleRepo := articlerepo.NewArticleRepo(db)
	agentLogRepo := articlerepo.NewAgentLogRepo(db)
	agentLogRecorder := modart.NewAgentLogRecorder(agentLogRepo)
	articleSvc := articleapp.NewService(
		articleRepo,
		agentLogRepo,
		userSvc,
		articleagent.NewOrchestrator(
			einoModel,
			imgGen,
			articleagent.DefaultImageMethodGuides(),
			agentLogRecorder,
		),
		ssehub,
		statsSvc,
	)

	e.Use(session.Middleware(store))

	docsui.Register(e)

	httpapi.RegisterRouter(e,
		authapi.NewRegistrar(authSvc),
		userapi.NewRegistrar(userSvc),
		articleapi.NewRegistrar(articleSvc, userSvc),
		paymentapi.NewRegistrar(paymentSvc, userSvc),
		statisticsapi.NewRegistrar(statsSvc, userSvc),
	)

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
