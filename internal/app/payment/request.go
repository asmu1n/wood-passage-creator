package payment

import (
	modpay "wood-passage-creator/internal/module/payment"
	"wood-passage-creator/internal/pkg/page"
)

// MockCompleteRequest mock 支付成功回调入参。
type MockCompleteRequest struct {
	SessionID string `json:"sessionId" validate:"required,min=8,max=128"`
}

// ListRequest 管理端支付记录分页查询。
type ListRequest struct {
	Status      *modpay.RecordStatus `query:"status" validate:"omitempty,oneof=PENDING SUCCEEDED FAILED REFUNDED"`
	UserID      *int64               `query:"userId" validate:"omitempty,gt=0"`
	ProductType *string              `query:"productType" validate:"omitempty,max=32"`
	page.PageRequest
}

// ListByUserRequest 当前用户支付记录分页查询。
type ListByUserRequest struct {
	Status      *modpay.RecordStatus `query:"status" validate:"omitempty,oneof=PENDING SUCCEEDED FAILED REFUNDED"`
	ProductType *string              `query:"productType" validate:"omitempty,max=32"`
	page.PageRequest
}

// RecordListData 支付记录分页 data 形态，供 swag 展示。
type RecordListData struct {
	Records  []*modpay.Record `json:"records"`
	Total    int              `json:"total"`
	PageSize int              `json:"pageSize"`
	PageNum  int              `json:"pageNum"`
}
