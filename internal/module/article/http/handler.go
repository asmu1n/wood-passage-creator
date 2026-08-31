package http

import (
	"net/http"
	"strconv"
	"time"
	"wood-passage-creator/internal/httpapi/binding"
	"wood-passage-creator/internal/httpapi/middleware"
	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/pkg/page"
	"wood-passage-creator/internal/pkg/response"
	pkgsse "wood-passage-creator/internal/pkg/sse"

	"github.com/labstack/echo/v5"
)

// Handler 文章 HTTP 传输层。
type Handler struct {
	svc *article.Service
}

func NewHandler(svc *article.Service) *Handler {
	return &Handler{svc: svc}
}

// Create godoc
// @Summary      创建文章生成任务
// @Description  登录用户创建一篇文章任务，进入生成流水线（具体异步逻辑由服务层处理）。
// @Tags         article
// @Accept       json
// @Produce      json
// @Param        body body article.CreateArticleRequest true "创建参数"
// @Success      200 {object} response.Response{data=article.Article} "成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      401 {object} response.Response "未登录"
// @Security     SessionAuth
// @Router       /article/create [post]
func (h *Handler) Create(c *echo.Context) error {
	var req article.CreateArticleRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	actorID, err := middleware.GetLoginUserID(c)
	if err != nil {
		return err
	}
	role, err := middleware.GetLoginUserRole(c)
	if err != nil {
		return err
	}
	u, err := h.svc.Create(c.Request().Context(), actorID, role, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(u))
}

// ConfirmTitle godoc
// @Summary      确认文章标题
// @Description  在 TITLE_SELECTING 阶段确认主/副标题；仅文章作者或管理员可操作。
// @Tags         article
// @Accept       json
// @Produce      json
// @Param        body body article.ConfirmTitleRequest true "确认标题参数"
// @Success      200 {object} response.Response "成功（data 一般为 null）"
// @Failure      400 {object} response.Response "参数错误/阶段不允许"
// @Failure      401 {object} response.Response "未登录"
// @Failure      403 {object} response.Response "无权限"
// @Failure      404 {object} response.Response "文章不存在"
// @Security     SessionAuth
// @Router       /article/confirm-title [post]
func (h *Handler) ConfirmTitle(c *echo.Context) error {
	var req article.ConfirmTitleRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	actorID, err := middleware.GetLoginUserID(c)
	if err != nil {
		return err
	}
	actorRole, err := middleware.GetLoginUserRole(c)
	if err != nil {
		return err
	}
	if err := h.svc.ConfirmTitle(c.Request().Context(), actorID, actorRole, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(nil))
}

// ConfirmOutline godoc
// @Summary      确认文章大纲
// @Description  在 OUTLINE_EDITING 阶段确认大纲；仅文章作者或管理员可操作。
// @Tags         article
// @Accept       json
// @Produce      json
// @Param        body body article.ConfirmOutlineRequest true "确认大纲参数"
// @Success      200 {object} response.Response "成功（data 一般为 null）"
// @Failure      400 {object} response.Response "参数错误/阶段不允许"
// @Failure      401 {object} response.Response "未登录"
// @Failure      403 {object} response.Response "无权限"
// @Failure      404 {object} response.Response "文章不存在"
// @Security     SessionAuth
// @Router       /article/confirm-outline [post]
func (h *Handler) ConfirmOutline(c *echo.Context) error {
	var req article.ConfirmOutlineRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	actorID, err := middleware.GetLoginUserID(c)
	if err != nil {
		return err
	}
	actorRole, err := middleware.GetLoginUserRole(c)
	if err != nil {
		return err
	}
	if err := h.svc.ConfirmOutline(c.Request().Context(), req.TaskID, actorID, actorRole, req.Outline); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(nil))
}

// GetByTaskID godoc
// @Summary      按任务 ID 查询文章
// @Description  使用创建任务时的 taskId 查询文章详情。
// @Tags         article
// @Produce      json
// @Param        taskId path string true "任务 ID（taskId）"
// @Success      200 {object} response.Response{data=article.Article} "成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      401 {object} response.Response "未登录"
// @Failure      404 {object} response.Response "文章不存在"
// @Security     SessionAuth
// @Router       /article/{taskId} [get]
func (h *Handler) GetByTaskID(c *echo.Context) error {
	// 与 register.go 路由 :taskId 保持一致
	taskID := c.Param("taskId")
	if taskID == "" {
		return response.NewBizErrorWithDetail(response.ParamsError, "任务ID不能为空")
	}
	actorID, err := middleware.GetLoginUserID(c)
	if err != nil {
		return err
	}
	actorRole, err := middleware.GetLoginUserRole(c)
	if err != nil {
		return err
	}
	u, err := h.svc.GetByTaskID(c.Request().Context(), taskID, actorID, actorRole)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(u))
}

// GetByID 按主键 ID 查询文章。
// 当前 register.go 未挂载该路由；保留 handler 供后续接入，故暂不生成 @Router。
func (h *Handler) GetByID(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.NewBizErrorWithDetail(response.ParamsError, "无效的文章 ID")
	}
	u, err := h.svc.GetByID(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(u))
}

// ListByUser 分页查询当前登录用户的文章。
// 当前 register.go 未挂载该路由；保留 handler 供后续接入，故暂不生成 @Router。
func (h *Handler) ListByUser(c *echo.Context) error {
	var req article.QueryArticleRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}

	actorID, err := middleware.GetLoginUserID(c)
	if err != nil {
		return err
	}
	u, total, err := h.svc.ListByUser(c.Request().Context(), actorID, req.PageRequest)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(page.NewPageResponse(u, total, req.PageRequest)))
}

// ListAll godoc
// @Summary      分页查询全部文章
// @Description  管理端/全量列表；具体权限由路由中间件控制。
// @Tags         article
// @Accept       json
// @Produce      json
// @Param        body body article.QueryArticleRequest true "分页与筛选（pageNum/pageSize/status）"
// @Success      200 {object} response.Response{data=article.ArticleListData} "成功"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      401 {object} response.Response "未登录"
// @Failure      403 {object} response.Response "无权限"
// @Security     SessionAuth
// @Router       /article/list [post]
func (h *Handler) ListAll(c *echo.Context) error {
	var req article.QueryArticleRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}
	u, total, err := h.svc.ListAll(c.Request().Context(), req.PageRequest)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(page.NewPageResponse(u, total, req.PageRequest)))
}

// Delete godoc
// @Summary      删除文章（软删除）
// @Description  按主键软删除文章。
// @Tags         article
// @Produce      json
// @Param        id query int true "文章主键 ID"
// @Success      200 {object} response.Response "成功（data 一般为 null）"
// @Failure      400 {object} response.Response "参数错误"
// @Failure      401 {object} response.Response "未登录"
// @Failure      404 {object} response.Response "文章不存在"
// @Security     SessionAuth
// @Router       /article/delete [post]
func (h *Handler) Delete(c *echo.Context) error {
	// 路由为 POST /article/delete，ID 从 query 读取（与 path 参数 :id 解耦）
	id, err := strconv.ParseInt(c.QueryParam("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.NewBizErrorWithDetail(response.ParamsError, "无效的文章 ID")
	}
	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(nil))
}

// GetProgress godoc
// @Summary      SSE 订阅文章生成进度
// @Description  按 taskId 建立 text/event-stream；需先登录且有权访问该任务。使用 SSE named events：connected / outline_delta / outline_done / task_error。
// @Tags         article
// @Produce      text/event-stream
// @Param        taskId path string true "任务 ID"
// @Success      200 {string} string "SSE 流"
// @Failure      401 {object} response.Response "未登录"
// @Failure      403 {object} response.Response "无权限"
// @Failure      404 {object} response.Response "文章不存在"
// @Security     SessionAuth
// @Router       /article/progress/{taskId} [get]
func (h *Handler) GetProgress(c *echo.Context) error {
	taskID := c.Param("taskId")
	if taskID == "" {
		return response.NewBizErrorWithDetail(response.ParamsError, "任务ID不能为空")
	}

	actorID, err := middleware.GetLoginUserID(c)
	if err != nil {
		return err
	}
	actorRole, err := middleware.GetLoginUserRole(c)
	if err != nil {
		return err
	}

	// 鉴权 + 订阅在 Service；失败时尚未进入 event-stream
	ch, cancel, err := h.svc.SubscribeProgress(
		c.Request().Context(),
		taskID,
		actorID,
		actorRole,
	)
	if err != nil {
		return err
	}
	defer cancel()

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)
	ctx := c.Request().Context()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	// 已开始 SSE 流后，写/刷新失败一律视为客户端断开，返回 nil，避免框架再写 JSON 错误体。
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-heartbeat.C:
			if err := pkgsse.WriteComment(w, "ping"); err != nil {
				return nil
			}
			if err := pkgsse.Flush(rc); err != nil {
				return nil
			}
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if err := pkgsse.WriteEvent(w, msg.Name, msg.Data); err != nil {
				return nil
			}
			if err := pkgsse.Flush(rc); err != nil {
				return nil
			}

			// 本段终态：结束 SSE 连接
			if article.IsTerminalSSEEvent(msg.Name) {
				return nil
			}

		}
	}
}

// ModifyOutline godoc
// @Summary      AI 修改大纲（VIP）
// @Description  仅 VIP/管理员；阶段须为 OUTLINE_EDITING。按自然语言建议改大纲并落库。
// @Tags         article
// @Accept       json
// @Produce      json
// @Param        body body article.AiModifyOutlineRequest true "任务 ID 与修改建议"
// @Success      200 {object} response.Response{data=[]article.OutlineSection}
// @Failure      400 {object} response.Response
// @Failure      401 {object} response.Response
// @Failure      403 {object} response.Response "非 VIP 或阶段不允许"
// @Security     SessionAuth
// @Router       /article/modify-outline [post]
func (h *Handler) ModifyOutline(c *echo.Context) error {
	var req article.AiModifyOutlineRequest
	if err := binding.BindAndValidate(c, &req); err != nil {
		return err
	}

	actorID, err := middleware.GetLoginUserID(c)
	if err != nil {
		return err
	}
	actorRole, err := middleware.GetLoginUserRole(c)
	if err != nil {
		return err
	}

	u, err := h.svc.ModifyOutline(c.Request().Context(), actorID, actorRole, req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, response.OK(u))
}

// GetExecutionLogs godoc
// @Summary      任务执行日志
// @Description  返回该 task 下各 agent 步骤的耗时与状态汇总。
// @Tags         article
// @Produce      json
// @Param        taskId path string true "任务 ID"
// @Success      200 {object} response.Response{data=article.AgentExecutionStats}
// @Security     SessionAuth
// @Router       /article/execution-logs/{taskId} [get]
func (h *Handler) GetExecutionLogs(c *echo.Context) error {
	taskID := c.Param("taskId")
	if taskID == "" {
		return response.NewBizErrorWithDetail(response.ParamsError, "任务ID不能为空")
	}
	actorID, err := middleware.GetLoginUserID(c)
	if err != nil {
		return err
	}
	actorRole, err := middleware.GetLoginUserRole(c)
	if err != nil {
		return err
	}
	stats, err := h.svc.GetExecutionLogs(c.Request().Context(), taskID, actorID, actorRole)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, response.OK(stats))
}
