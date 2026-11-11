package articleapp

import (
	"context"
	"testing"

	module "wood-passage-creator/internal/module/article"
	moduser "wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/logger"
	"wood-passage-creator/internal/pkg/response"
)

type conflictingPhaseRepo struct {
	module.Repository
	stored   module.Article
	expected module.ArticlePhase
	next     module.ArticlePhase
}

func (r *conflictingPhaseRepo) GetByTaskID(context.Context, string) (*module.Article, error) {
	copy := r.stored
	return &copy, nil
}
func (r *conflictingPhaseRepo) UpdateIfPhase(_ context.Context, _ int64, expected module.ArticlePhase, params module.UpdateArticleParams) (*module.Article, error) {
	r.expected = expected
	if params.Phase != nil {
		r.next = *params.Phase
	}
	return nil, nil
}

func TestConfirmationsRejectLostAtomicTransition(t *testing.T) {
	for _, phase := range []module.ArticlePhase{module.PhaseTitleSelecting, module.PhaseOutlineEditing} {
		t.Run(string(phase), func(t *testing.T) {
			repo := &conflictingPhaseRepo{stored: module.Article{ID: 1, TaskID: "task", UserID: 1, Phase: phase}}
			service := &Service{repo: repo, log: logger.Module("test")}
			actor := moduser.Actor{ID: 1, Role: moduser.RoleUser}
			var err error
			if phase == module.PhaseTitleSelecting {
				err = service.ConfirmTitle(t.Context(), actor, ConfirmTitleRequest{TaskID: "task", SelectedMainTitle: "Main", SelectedSubTitle: "Sub"})
				if repo.next != module.PhaseOutlineGenerating {
					t.Fatal("title confirmation did not atomically set next phase")
				}
			} else {
				err = service.ConfirmOutline(t.Context(), actor, "task", []module.OutlineSection{{Section: 1, Title: "Intro", Points: []string{"Point"}}})
				if repo.next != module.PhaseContentGenerating {
					t.Fatal("outline confirmation did not atomically set next phase")
				}
			}
			assertBizCode(t, err, response.Forbidden.Biz)
			if repo.expected != phase {
				t.Fatal("confirmation did not include expected phase")
			}
		})
	}
}

type executionLogRepo struct {
	module.AgentLogRepository
	logs []*module.AgentLog
}

func (r executionLogRepo) ListByTaskID(context.Context, string) ([]*module.AgentLog, error) {
	return r.logs, nil
}

func TestRepeatedOutlineRevisionsAccumulateBusinessDurations(t *testing.T) {
	repo := &conflictingPhaseRepo{stored: module.Article{ID: 1, TaskID: "task", UserID: 1}}
	service := &Service{repo: repo, agentLogs: executionLogRepo{logs: []*module.AgentLog{
		{AgentName: "outline_modifier", DurationMs: new(20), Status: module.AgentLogSuccess},
		{AgentName: "outline_modifier", DurationMs: new(30), Status: module.AgentLogSuccess},
	}}}
	stats, err := service.GetExecutionLogs(t.Context(), moduser.Actor{ID: 1, Role: moduser.RoleUser}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if stats.AgentCount != 2 || stats.TotalDurationMs != 50 || stats.AgentDurations["outline_modifier"] != 50 {
		t.Fatalf("revisions were overwritten in the execution summary: %+v", stats)
	}
}
