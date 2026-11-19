package articleapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	articleapp "wood-passage-creator/internal/app/article"
	"wood-passage-creator/internal/app/article/workflow"
	"wood-passage-creator/internal/httpapi/middleware"
	module "wood-passage-creator/internal/module/article"
	moduser "wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/sse"
)

type snapshotRepo struct {
	module.Repository
	stored module.Article
}

func (r snapshotRepo) GetByTaskID(context.Context, string) (*module.Article, error) {
	copy := r.stored
	return &copy, nil
}

func TestProgressHandlerClosesForPersistedConfirmationAndCompletion(t *testing.T) {
	for _, phase := range []module.ArticlePhase{module.PhaseTitleSelecting, module.PhaseOutlineEditing, module.PhaseCompleted} {
		t.Run(string(phase), func(t *testing.T) {
			repo := snapshotRepo{stored: module.Article{ID: 1, TaskID: "task", UserID: 1, Phase: phase, Status: module.StatusProcessing}}
			if phase == module.PhaseCompleted {
				repo.stored.Status = module.StatusCompleted
			}
			service := articleapp.NewService(repo, nil, nil, nil, nil, nil, sse.NewHub(), nil)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			request := httptest.NewRequest("GET", "/api/article/progress/task", nil).WithContext(ctx)
			w := httptest.NewRecorder()
			c := echo.New().NewContext(request, w)
			c.SetPathValues(echo.PathValues{{Name: "taskId", Value: "task"}})
			c.Set(middleware.ContextKeyLoginActor, moduser.Actor{ID: 1, Role: moduser.RoleUser})
			if err := NewHandler(service).GetProgress(c); err != nil {
				t.Fatal(err)
			}
			if ctx.Err() != nil {
				t.Fatal("handler waited for cancellation instead of closing a terminal snapshot")
			}
			if w.Header().Get("Content-Type") != "text/event-stream" || !strings.HasPrefix(w.Body.String(), "event: task_snapshot\n") {
				t.Fatalf("unexpected SSE response: %s", w.Body.String())
			}
			data := strings.TrimSpace(strings.TrimPrefix(strings.SplitN(w.Body.String(), "data: ", 2)[1], "data: "))
			var envelope articleworkflow.Event[articleworkflow.TaskSnapshot]
			if err := json.Unmarshal([]byte(data), &envelope); err != nil {
				t.Fatal(err)
			}
			if !envelope.Data.Terminal || envelope.Phase != phase || envelope.RunID == "" {
				t.Fatalf("invalid snapshot: %+v", envelope)
			}
		})
	}
}

func TestProgressHandlerAuthorizesBeforeWritingStream(t *testing.T) {
	repo := snapshotRepo{stored: module.Article{ID: 1, TaskID: "task", UserID: 2, Phase: module.PhaseCompleted, Status: module.StatusCompleted}}
	service := articleapp.NewService(repo, nil, nil, nil, nil, nil, sse.NewHub(), nil)
	w := httptest.NewRecorder()
	c := echo.New().NewContext(httptest.NewRequest("GET", "/api/article/progress/task", nil), w)
	c.SetPathValues(echo.PathValues{{Name: "taskId", Value: "task"}})
	c.Set(middleware.ContextKeyLoginActor, moduser.Actor{ID: 1, Role: moduser.RoleUser})
	if err := NewHandler(service).GetProgress(c); err == nil {
		t.Fatal("unauthorized progress access accepted")
	}
	if w.Body.Len() != 0 || w.Header().Get("Content-Type") == "text/event-stream" {
		t.Fatal("stream started before authorization")
	}
}
