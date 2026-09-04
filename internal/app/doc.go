// Package app 是唯一应用用例层：全部业务用例与 *Request 入参放在这里。
//
//	httpapi/api → app.<usecase>.Service（Bind app.*Request）
//	app         → module 的实体 / Repository 接口 / 领域工具
//	module/*/repo → ent（ClientFrom）
//
// module 不再提供 Service。跨域强一致写用 port.WithinTx（全局 TxManager；如创建文章扣配额、支付履约升 VIP）。
package app
