package payment

import "context"

// Repository 支付记录仓储。
type Repository interface {
	CreatePending(ctx context.Context, userID int64, sessionID, productType, currency string, amount float64, desc string) (*Record, error)
	GetBySessionID(ctx context.Context, sessionID string) (*Record, error)
	MarkSucceeded(ctx context.Context, id int64, paymentIntentID string) (*Record, error)
	List(ctx context.Context, params ListParams) (items []*Record, total int, err error)
}

type ListParams struct {
	AdminListRequest
}
