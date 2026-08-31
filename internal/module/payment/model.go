package payment

import "time"

// 状态与 product 常量（对齐 ent enum / 旧项目）。
const (
	StatusPending   = "PENDING"
	StatusSucceeded = "SUCCEEDED"
	StatusFailed    = "FAILED"
	StatusRefunded  = "REFUNDED"

	ProductVIPPermanent = "VIP_PERMANENT"

	// 开发态 mock 金额（美元展示用，非真实扣款）。
	MockVIPAmount   = 29.9
	MockVIPCurrency = "usd"
)

// Record 支付记录（API 可见）。
type Record struct {
	ID                    int64      `json:"id"`
	UserID                int64      `json:"userId"`
	StripeSessionID       string     `json:"stripeSessionId"`
	StripePaymentIntentID *string    `json:"stripePaymentIntentId,omitempty"`
	Amount                float64    `json:"amount"`
	Currency              string     `json:"currency"`
	Status                string     `json:"status"`
	ProductType           string     `json:"productType"`
	Description           *string    `json:"description,omitempty"`
	CreateTime            time.Time  `json:"createTime"`
	UpdateTime            time.Time  `json:"updateTime"`
}

// MockSessionResult 创建 mock 会话的返回。
type MockSessionResult struct {
	SessionID   string  `json:"sessionId"`
	CheckoutURL string  `json:"checkoutUrl"` // 假地址，仅便于前端联调展示
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	ProductType string  `json:"productType"`
	Status      string  `json:"status"`
}

// MockCompleteRequest mock 支付成功回调入参。
type MockCompleteRequest struct {
	SessionID string `json:"sessionId" validate:"required,min=8,max=128"`
}

// MockCompleteResult 完成后的摘要。
type MockCompleteResult struct {
	Record *Record `json:"record"`
	UserID int64   `json:"userId"`
	IsVIP  bool    `json:"isVip"`
}
