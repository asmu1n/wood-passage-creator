import request from '@/request'

/** 创建 VIP mock 支付会话 POST /payment/vip/mock-session */
export async function createVipPaymentSession(options?: { [key: string]: any }) {
  return request<API.BaseResponseMockSessionResult>('/payment/vip/mock-session', {
    method: 'POST',
    ...(options || {}),
  })
}

/** 完成 VIP mock 支付 POST /payment/vip/mock-complete */
export async function completeMockVipPayment(
  body: API.MockCompleteRequest,
  options?: { [key: string]: any },
) {
  return request<API.BaseResponseMockCompleteResult>('/payment/vip/mock-complete', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    data: body,
    ...(options || {}),
  })
}

/** 当前用户支付记录 GET /payment/list */
export async function getPaymentRecords(
  params: API.PageQuery = {},
  options?: { [key: string]: any },
) {
  return request<API.BaseResponsePagePaymentRecord>('/payment/list', {
    method: 'GET',
    params: {
      pageNum: params.pageNum,
      pageSize: params.pageSize,
      status: params.status,
      productType: params.productType,
    },
    ...(options || {}),
  })
}

/** 管理端支付记录 GET /admin/payment/list */
export async function adminListPaymentRecords(
  params: API.PageQuery & { userId?: number } = {},
  options?: { [key: string]: any },
) {
  return request<API.BaseResponsePagePaymentRecord>('/admin/payment/list', {
    method: 'GET',
    params: {
      pageNum: params.pageNum,
      pageSize: params.pageSize,
      status: params.status,
      productType: params.productType,
      userId: params.userId,
    },
    ...(options || {}),
  })
}
