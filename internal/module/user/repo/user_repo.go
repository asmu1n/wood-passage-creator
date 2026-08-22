package repo

import (
	"context"
	"time"

	"wood-passage-creator/ent"
	entgen "wood-passage-creator/ent/user"
	"wood-passage-creator/internal/module/user"
	"wood-passage-creator/internal/pkg/page"
)

type UserRepo struct {
	client *ent.Client
}

func New(client *ent.Client) user.Repository {
	return &UserRepo{client: client}
}

func (r *UserRepo) Create(ctx context.Context, in user.CreateRepoParams) (*user.User, error) {
	b := r.client.User.Create().
		SetUserAccount(in.UserAccount).
		SetUserPassword(in.UserPassword).
		SetUserRole(toEntRole(in.UserRole)).
		SetQuota(in.Quota)

	if in.UserName != nil {
		b.SetUserName(*in.UserName)
	}
	if in.UserAvatar != nil {
		b.SetUserAvatar(*in.UserAvatar)
	}
	if in.UserProfile != nil {
		b.SetUserProfile(*in.UserProfile)
	}

	row, err := b.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, user.ErrAccountConflict
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r *UserRepo) QueryList(ctx context.Context, params page.PageRequest) ([]*user.User, int, error) {
	base := r.client.User.Query().
		Where(entgen.IsDeleteEQ(false))

	count, err := base.Clone().Count(ctx)

	rows, err := base.Clone().
		Offset(int(params.Offset())).
		Limit(int(params.Limit())).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return toDomainList(rows), count, nil
}

func (r *UserRepo) FindByID(ctx context.Context, id int64) (*user.User, error) {
	row, err := r.client.User.Query().
		Where(
			entgen.IDEQ(id),
			entgen.IsDeleteEQ(false),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r *UserRepo) FindByAccount(ctx context.Context, account string) (*user.UserWithSecret, error) {
	row, err := r.client.User.Query().
		Where(
			entgen.UserAccountEQ(account),
			entgen.IsDeleteEQ(false),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	u := toDomain(row)
	return &user.UserWithSecret{User: *u, UserPassword: row.UserPassword}, nil
}

func (r *UserRepo) Update(ctx context.Context, id int64, in user.UpdateRepoParams) (*user.User, error) {
	b := r.client.User.UpdateOneID(id).
		Where(entgen.IsDeleteEQ(false)).
		SetEditTime(time.Now())

	if in.UserPassword != nil {
		b.SetUserPassword(*in.UserPassword)
	}
	if in.UserName != nil {
		b.SetUserName(*in.UserName)
	}
	if in.UserAvatar != nil {
		b.SetUserAvatar(*in.UserAvatar)
	}
	if in.UserProfile != nil {
		b.SetUserProfile(*in.UserProfile)
	}
	if in.UserRole != nil {
		b.SetUserRole(toEntRole(*in.UserRole))
	}
	if in.Quota != nil {
		b.SetQuota(*in.Quota)
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

func (r *UserRepo) ExistsAccount(ctx context.Context, account string) (bool, error) {
	return r.client.User.Query().
		Where(
			entgen.UserAccountEQ(account),
			entgen.IsDeleteEQ(false),
		).
		Exist(ctx)
}

func (r *UserRepo) Delete(ctx context.Context, id int64) error {
	n, err := r.client.User.Update().
		Where(entgen.IDEQ(id), entgen.IsDeleteEQ(false)).
		SetIsDelete(true).
		SetEditTime(time.Now()).
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	return nil
}

func toDomain(row *ent.User) *user.User {
	return &user.User{
		ID:          row.ID,
		UserAccount: row.UserAccount,
		UserName:    row.UserName,
		UserAvatar:  row.UserAvatar,
		UserProfile: row.UserProfile,
		UserRole:    user.UserRole(row.UserRole),
		Quota:       row.Quota,
		VipTime:     row.VipTime,
		EditTime:    row.EditTime,
		CreateTime:  row.CreateTime,
		UpdateTime:  row.UpdateTime,
	}
}

func toDomainList(rows []*ent.User) []*user.User {
	users := make([]*user.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, toDomain(row))
	}
	return users
}

func toEntRole(r user.UserRole) entgen.UserRole {
	switch r {
	case user.RoleAdmin:
		return entgen.UserRoleAdmin
	case user.RoleVIP:
		return entgen.UserRoleVip
	default:
		return entgen.UserRoleUser
	}
}
