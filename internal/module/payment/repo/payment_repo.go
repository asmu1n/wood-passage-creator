package repo

import (
	"context"

	"wood-passage-creator/ent"
	entgen "wood-passage-creator/ent/paymentrecord"
	"wood-passage-creator/internal/module/payment"
)

type PaymentRepo struct {
	client *ent.Client
}

func New(client *ent.Client) payment.Repository {
	return &PaymentRepo{client: client}
}

func (r *PaymentRepo) CreatePending(ctx context.Context, userID int64, sessionID, productType, currency string, amount float64, desc string) (*payment.Record, error) {
	b := r.client.PaymentRecord.Create().
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

func (r *PaymentRepo) GetBySessionID(ctx context.Context, sessionID string) (*payment.Record, error) {
	row, err := r.client.PaymentRecord.Query().
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

func (r *PaymentRepo) MarkSucceeded(ctx context.Context, id int64, paymentIntentID string) (*payment.Record, error) {
	b := r.client.PaymentRecord.UpdateOneID(id).
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
		Status:                string(row.Status),
		ProductType:           row.ProductType,
		Description:           row.Description,
		CreateTime:            row.CreateTime,
		UpdateTime:            row.UpdateTime,
	}
}
