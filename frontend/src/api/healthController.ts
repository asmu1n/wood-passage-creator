import axios from 'axios'

/**
 * 健康检查在后端根路径 GET /health（不在 /api 下）。
 * 开发态经 vite 需额外代理 /health，或直连后端 origin。
 */
export async function healthCheck(options?: { [key: string]: any }) {
  const base = (import.meta.env.VITE_API_BASE_URL as string) || '/api'
  // /api -> 同源 /health；完整 URL 则退到 origin
  let url = '/health'
  if (/^https?:\/\//i.test(base)) {
    try {
      url = new URL('/health', base).toString()
    } catch {
      url = '/health'
    }
  }
  return axios.get<{ status: string }>(url, {
    withCredentials: true,
    ...(options || {}),
  })
}
