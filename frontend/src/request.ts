import axios, { type AxiosError } from 'axios'
import { message } from 'ant-design-vue'
import { API_BASE_URL } from '@/config/env'
import { REQUEST_TIMEOUT, UNAUTHORIZED_CODE } from '@/constants'

const myAxios = axios.create({
  baseURL: API_BASE_URL,
  timeout: REQUEST_TIMEOUT,
  withCredentials: true,
})

function isAuthMeURL(url?: string) {
  if (!url) return false
  return url.includes('/auth/me') || url.includes('user/get/login')
}

function redirectLogin() {
  if (window.location.pathname.includes('/user/login')) return
  message.warning('请先登录')
  const redirect = encodeURIComponent(window.location.href)
  window.location.href = `/user/login?redirect=${redirect}`
}

myAxios.interceptors.request.use(
  (config) => config,
  (error) => Promise.reject(error),
)

myAxios.interceptors.response.use(
  (response) => {
    const { data } = response
    // 兼容仍返回 HTTP 200 + 业务码的路径
    if (data && typeof data === 'object' && data.code === UNAUTHORIZED_CODE) {
      if (!isAuthMeURL(response.config?.url) && !isAuthMeURL(response.request?.responseURL)) {
        redirectLogin()
      }
    }
    return response
  },
  (error: AxiosError<API.BaseResponse>) => {
    const status = error.response?.status
    const body = error.response?.data

    if (status === 401 || body?.code === UNAUTHORIZED_CODE) {
      if (!isAuthMeURL(error.config?.url)) {
        redirectLogin()
      }
    } else if (body?.message) {
      // 业务错误：把 message 挂到 Error 上，页面 catch 可直接展示
      const err = new Error(body.message) as Error & { code?: number; response?: typeof error.response }
      err.code = body.code
      err.response = error.response
      return Promise.reject(err)
    }
    return Promise.reject(error)
  },
)

export default myAxios
