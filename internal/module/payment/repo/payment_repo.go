package repo

import (
	"context"

	"wood-passage-creator/ent"
	entgen "wood-passage-creator/ent/paymentrecord"
	"wood-passage-creator/internal/infra/database"
	"wood-passage-creator/internal/module/payment"

	"github.com/samber/lo"
)

type repo struct {
	database.DB
}

func New(db database.DB) payment.Repository {
	return &repo{db}
}

func (r *repo) CreatePending(ctx context.Context, userID int64, sessionID, productType, currency string, amount float64, desc string) (*payment.Record, error) {
	b := r.Cli(ctx).PaymentRecord.Create().
		SetUserID(userID).
		SetStripeSessionID(sessionID).
		SetAmount(amount).
		SetCurrency(currency).
		SetStatus(entgen.StatusPENDING).
		SetProductType(productType)
	if desc != "" {
		b.SetDescription(desc)
	}
	row, err := b.Save(ctx)
	if err != nil {
		return nil, err
	}
	return toDomain(row), nil
}

func (r *repo) GetBySessionID(ctx context.Context, sessionID string) (*payment.Record, error) {
	row, err := r.Cli(ctx).PaymentRecord.Query().
		Where(entgen.StripeSessionIDEQ(sessionID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r *repo) MarkSucceeded(ctx context.Context, id int64, paymentIntentID string) (*payment.Record, error) {
	b := r.Cli(ctx).PaymentRecord.UpdateOneID(id).
		SetStatus(entgen.StatusSUCCEEDED)
	if paymentIntentID != "" {
		b.SetStripePaymentIntentID(paymentIntentID)
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

func (r *repo) List(ctx context.Context, params payment.ListParams) ([]*payment.Record, int, error) {
	base := r.Cli(ctx).PaymentRecord.Query()
	if params.UserID != nil {
		base = base.Where(entgen.UserIDEQ(*params.UserID))
	}
	if params.Status != nil {
		base = base.Where(entgen.StatusEQ(entgen.Status(*params.Status)))
	}
	if params.ProductType != nil {
		base = base.Where(entgen.ProductTypeEQ(*params.ProductType))
	}

	count, err := base.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	rows, err := base.Limit(params.Limit()).Offset(params.Offset()).Order(ent.Desc(entgen.FieldCreateTime)).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	return toDomainList(rows), count, nil
}

func toDomain(row *ent.PaymentRecord) *payment.Record {
	if row == nil {
		return nil
	}
	return &payment.Record{
		ID:                    row.ID,
		UserID:                row.UserID,
		StripeSessionID:       row.StripeSessionID,
		StripePaymentIntentID: row.StripePaymentIntentID,
		Amount:                row.Amount,
		Currency:              row.Currency,
		Status:                payment.RecordStatus(row.Status),
		ProductType:           row.ProductType,
		Description:           row.Description,
		CreateTime:            row.CreateTime,
		UpdateTime:            row.UpdateTime,
	}
}

func toDomainList(rows []*ent.PaymentRecord) []*payment.Record {
	return lo.Map(rows, func(row *ent.PaymentRecord, _ int) *payment.Record {
		return toDomain(row)
	})
}
