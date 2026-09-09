import request from '@/request'

/** 注册 POST /auth/register */
export async function userRegister(body: API.UserRegisterRequest, options?: { [key: string]: any }) {
  return request<API.BaseResponseUser>('/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    data: body,
    ...(options || {}),
  })
}

/** 登录 POST /auth/login */
export async function userLogin(body: API.UserLoginRequest, options?: { [key: string]: any }) {
  return request<API.BaseResponseUser>('/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    data: body,
    ...(options || {}),
  })
}

/** 登出 POST /auth/logout */
export async function userLogout(options?: { [key: string]: any }) {
  return request<API.BaseResponse>('/auth/logout', {
    method: 'POST',
    ...(options || {}),
  })
}

/** 当前登录用户 GET /auth/me */
export async function getLoginUser(options?: { [key: string]: any }) {
  return request<API.BaseResponseUser>('/auth/me', {
    method: 'GET',
    ...(options || {}),
  })
}
