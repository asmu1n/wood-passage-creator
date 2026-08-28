package repo

import (
	"context"

	"wood-passage-creator/ent"
	entlog "wood-passage-creator/ent/agentlog"
	"wood-passage-creator/internal/module/article"
)

type AgentLogRepo struct {
	client *ent.Client
}

func NewAgentLogRepo(client *ent.Client) article.AgentLogRepository {
	return &AgentLogRepo{client: client}
}

func (r *AgentLogRepo) Create(ctx context.Context, params article.CreateAgentLogParams) error {
	b := r.client.AgentLog.Create().
		SetArticleID(params.ArticleID).
		SetTaskID(params.TaskID).
		SetAgentName(params.AgentName).
		SetStartTime(params.StartTime).
		SetEndTime(params.EndTime).
		SetDurationMs(params.DurationMs).
		SetStatus(entlog.Status(params.Status))

	if params.ErrorMessage != "" {
		b.SetErrorMessage(params.ErrorMessage)
	}
	if params.InputData != "" {
		b.SetInputData(params.InputData)
	}
	if params.OutputData != "" {
		b.SetOutputData(params.OutputData)
	}
	if params.Prompt != "" {
		b.SetPrompt(params.Prompt)
	}
	_, err := b.Save(ctx)
	return err
}

func (r *AgentLogRepo) ListByTaskID(ctx context.Context, taskID string) ([]*article.AgentLog, error) {
	rows, err := r.client.AgentLog.Query().
		Where(
			entlog.TaskIDEQ(taskID),
			entlog.IsDeleteEQ(false),
		).
		Order(ent.Asc(entlog.FieldCreateTime), ent.Asc(entlog.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*article.AgentLog, 0, len(rows))
	for _, row := range rows {
		out = append(out, agentLogToDomain(row))
	}
	return out, nil
}

func agentLogToDomain(row *ent.AgentLog) *article.AgentLog {
	if row == nil {
		return nil
	}
	return &article.AgentLog{
		ID:           row.ID,
		ArticleID:    row.ArticleID,
		TaskID:       row.TaskID,
		AgentName:    row.AgentName,
		StartTime:    row.StartTime,
		EndTime:      row.EndTime,
		DurationMs:   row.DurationMs,
		Status:       article.AgentLogStatus(row.Status),
		ErrorMessage: row.ErrorMessage,
		Prompt:       row.Prompt,
		InputData:    row.InputData,
		OutputData:   row.OutputData,
		CreateTime:   row.CreateTime,
	}
}
