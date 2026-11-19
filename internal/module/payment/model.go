package payment

import (
	"time"
)

type RecordStatus string

// 状态与 product 常量（对齐 ent enum / 旧项目）。
const (
	StatusPending   RecordStatus = "PENDING"
	StatusSucceeded RecordStatus = "SUCCEEDED"
	StatusFailed    RecordStatus = "FAILED"
	StatusRefunded  RecordStatus = "REFUNDED"

	ProductVIPPermanent = "VIP_PERMANENT"

	// 开发态 mock 金额（美元展示用，非真实扣款）。
	MockVIPAmount   = 29.9
	MockVIPCurrency = "usd"
)

// Record 支付记录（API 可见）。
type Record struct {
	ID                    int64        `json:"id"`
	UserID                int64        `json:"userId"`
	StripeSessionID       string       `json:"stripeSessionId"`
	StripePaymentIntentID *string      `json:"stripePaymentIntentId,omitempty"`
	Amount                float64      `json:"amount"`
	Currency              string       `json:"currency"`
	Status                RecordStatus `json:"status"`
	ProductType           string       `json:"productType"`
	Description           *string      `json:"description,omitempty"`
	CreateTime            time.Time    `json:"createTime"`
	UpdateTime            time.Time    `json:"updateTime"`
}

// MockSessionResult 创建 mock 会话的返回。
type MockSessionResult struct {
	SessionID   string       `json:"sessionId"`
	CheckoutURL string       `json:"checkoutUrl"`
	Amount      float64      `json:"amount"`
	Currency    string       `json:"currency"`
	ProductType string       `json:"productType"`
	Status      RecordStatus `json:"status"`
}

// MockCompleteResult 完成后的摘要。
type MockCompleteResult struct {
	Record *Record `json:"record"`
	UserID int64   `json:"userId"`
	IsVIP  bool    `json:"isVip"`
}
