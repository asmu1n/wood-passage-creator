package statistics

// Overview 管理端系统概览指标（对齐旧项目 StatisticsVO）。
type Overview struct {
	TodayCount      int64   `json:"todayCount"`
	WeekCount       int64   `json:"weekCount"`
	MonthCount      int64   `json:"monthCount"`
	TotalCount      int64   `json:"totalCount"`
	SuccessRate     float64 `json:"successRate"`     // 0~100
	AvgDurationMs   int     `json:"avgDurationMs"`
	ActiveUserCount int64   `json:"activeUserCount"` // 本周有创作的用户数
	TotalUserCount  int64   `json:"totalUserCount"`
	VipUserCount    int64   `json:"vipUserCount"`
	QuotaUsed       int64   `json:"quotaUsed"`
}

// DefaultUserQuota 普通用户注册初始配额（与 user.Register 一致，用于估算已用配额）。
const DefaultUserQuota = 5
