package repo

import (
	"context"
	"encoding/json"

	"wood-passage-creator/ent"
	entm "wood-passage-creator/ent/article"
	"wood-passage-creator/internal/module/article"
)

type ArticleRepo struct {
	client *ent.Client
}

func NewArticleRepo(client *ent.Client) article.Repository {
	return &ArticleRepo{client: client}
}

func (r *ArticleRepo) Create(ctx context.Context, params article.CreateArticleParams) (*article.Article, error) {
	row, err := r.client.Article.Create().
		SetUserID(params.UserID).
		SetTaskID(params.TaskID).
		SetTopic(params.Topic).
		SetEnabledImageMethods(params.EnabledImageMethods).
		SetNillableStyle((*entm.Style)(params.Style)).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return toDomain(row), nil
}

func (r *ArticleRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.client.Article.UpdateOneID(id).
		SetIsDelete(true).
		Save(ctx)
	return err
}

func (r *ArticleRepo) GetByID(ctx context.Context, id int64) (*article.Article, error) {
	row, err := r.client.Article.Query().
		Where(entm.IDEQ(id), entm.IsDeleteEQ(false)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r *ArticleRepo) GetByTaskID(ctx context.Context, taskID string) (*article.Article, error) {
	row, err := r.client.Article.Query().
		Where(entm.TaskIDEQ(taskID), entm.IsDeleteEQ(false)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r *ArticleRepo) Update(ctx context.Context, id int64, params article.UpdateArticleParams) (*article.Article, error) {
	b := r.client.Article.UpdateOneID(id).
		Where(entm.IsDeleteEQ(false))
	if err := applyUpdate(b, params); err != nil {
		return nil, err
	}
	row, err := b.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r *ArticleRepo) UpdateByTaskID(ctx context.Context, taskID string, params article.UpdateArticleParams) (*article.Article, error) {
	// 先定位主键，复用 UpdateOne 的返回实体；避免 Update().Save 只返回 affected。
	id, err := r.client.Article.Query().
		Where(entm.TaskIDEQ(taskID), entm.IsDeleteEQ(false)).
		OnlyID(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return r.Update(ctx, id, params)
}

func (r *ArticleRepo) UpdateStatus(ctx context.Context, taskID string, status article.ArticleStatus) error {
	_, err := r.UpdateByTaskID(ctx, taskID, article.UpdateArticleParams{
		Status: &status,
	})
	return err
}

func (r *ArticleRepo) UpdatePhase(ctx context.Context, taskID string, phase article.ArticlePhase) error {
	_, err := r.UpdateByTaskID(ctx, taskID, article.UpdateArticleParams{
		Phase: &phase,
	})
	return err
}

func (r *ArticleRepo) UpdateTitleOptions(ctx context.Context, taskID string, titleOptions []article.TitleOption) error {
	_, err := r.UpdateByTaskID(ctx, taskID, article.UpdateArticleParams{
		TitleOptions: titleOptions,
	})
	return err
}

func (r *ArticleRepo) UpdateOutline(ctx context.Context, taskID string, outline []article.OutlineSection) error {
	_, err := r.UpdateByTaskID(ctx, taskID, article.UpdateArticleParams{
		Outline: outline,
	})
	return err
}

func (r *ArticleRepo) UpdateSubTitle(ctx context.Context, taskID string, subTitle string) error {
	_, err := r.UpdateByTaskID(ctx, taskID, article.UpdateArticleParams{
		SubTitle: &subTitle,
	})
	return err
}

func (r *ArticleRepo) ListByUser(ctx context.Context, userID int64, params article.ListArticlesParams) ([]*article.Article, int, error) {
	// 基础查询只放过滤条件；Count 与分页列表必须分开，避免 Limit/Offset 污染 total。
	base := r.client.Article.Query().
		Where(entm.UserIDEQ(userID), entm.IsDeleteEQ(false))
	if params.Status != nil {
		base = base.Where(entm.StatusEQ(entm.Status(*params.Status)))
	}
	return listPage(ctx, base, params)
}

func (r *ArticleRepo) List(ctx context.Context, params article.ListArticlesParams) ([]*article.Article, int, error) {
	base := r.client.Article.Query().
		Where(entm.IsDeleteEQ(false))
	if params.Status != nil {
		base = base.Where(entm.StatusEQ(entm.Status(*params.Status)))
	}
	return listPage(ctx, base, params)
}

func listPage(ctx context.Context, base *ent.ArticleQuery, params article.ListArticlesParams) ([]*article.Article, int, error) {
	total, err := base.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := base.
		Order(ent.Desc(entm.FieldCreateTime)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return toDomainList(rows), total, nil
}

func applyUpdate(b *ent.ArticleUpdateOne, in article.UpdateArticleParams) error {
	b.SetNillableUserDescription(in.UserDescription)
	b.SetNillableMainTitle(in.MainTitle)
	b.SetNillableSubTitle(in.SubTitle)
	b.SetNillableContent(in.Content)
	b.SetNillableFullContent(in.FullContent)
	b.SetNillableErrorMessage(in.ErrorMessage)
	b.SetNillableCompletedTime(in.CompletedTime)
	b.SetNillableStyle((*entm.Style)(in.Style))
	if in.Status != nil {
		b.SetStatus(entm.Status(*in.Status))
	}
	if in.Phase != nil {
		b.SetPhase(entm.Phase(*in.Phase))
	}

	// —— JSON 字段：nil 切片 = 不改；
	if in.TitleOptions != nil {
		raw, err := json.Marshal(in.TitleOptions)
		if err != nil {
			return err
		}
		b.SetTitleOptions(raw)
	}
	if in.Outline != nil {
		raw, err := json.Marshal(in.Outline)
		if err != nil {
			return err
		}
		b.SetOutline(raw)
	}
	if in.Images != nil {
		raw, err := json.Marshal(in.Images)
		if err != nil {
			return err
		}
		b.SetImages(raw)
	}
	if in.EnabledImageMethods != nil {
		b.SetEnabledImageMethods(in.EnabledImageMethods)
	}

	return nil
}

func toDomain(row *ent.Article) *article.Article {
	if row == nil {
		return nil
	}
	out := &article.Article{
		ID:                  row.ID,
		TaskID:              row.TaskID,
		UserID:              row.UserID,
		Topic:               row.Topic,
		UserDescription:     row.UserDescription,
		MainTitle:           row.MainTitle,
		SubTitle:            row.SubTitle,
		Content:             row.Content,
		FullContent:         row.FullContent,
		Status:              article.ArticleStatus(row.Status),
		Phase:               article.ArticlePhase(row.Phase),
		ErrorMessage:        row.ErrorMessage,
		Style:               (*article.ArticleStyle)(&row.Style),
		EnabledImageMethods: row.EnabledImageMethods,
		CreateTime:          row.CreateTime,
		CompletedTime:       row.CompletedTime,
	}
	if len(row.TitleOptions) > 0 {
		_ = json.Unmarshal(row.TitleOptions, &out.TitleOptions)
	}
	if len(row.Outline) > 0 {
		_ = json.Unmarshal(row.Outline, &out.Outline)
	}
	if len(row.Images) > 0 {
		_ = json.Unmarshal(row.Images, &out.Images)
	}
	return out
}

func toDomainList(rows []*ent.Article) []*article.Article {
	list := make([]*article.Article, 0, len(rows))
	for _, row := range rows {
		list = append(list, toDomain(row))
	}
	return list
}
