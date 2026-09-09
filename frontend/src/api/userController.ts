import request from '@/request'

// 兼容旧 import：鉴权接口仍从本文件 re-export
export { userRegister, userLogin, userLogout, getLoginUser } from './authController'

/** 按 ID 获取用户 GET /users/:id */
export async function getUserById(
  params: API.getUserByIdParams,
  options?: { [key: string]: any },
) {
  return request<API.BaseResponseUser>(`/users/${params.id}`, {
    method: 'GET',
    ...(options || {}),
  })
}

/** 更新用户（本人或管理员） PATCH /users/:id */
export async function updateUser(
  id: number,
  body: API.UserUpdateRequest,
  options?: { [key: string]: any },
) {
  return request<API.BaseResponseUser>(`/users/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    data: body,
    ...(options || {}),
  })
}

/** 管理端用户分页 GET /admin/users/list */
export async function listUserVoByPage(
  params: API.UserQueryRequest,
  options?: { [key: string]: any },
) {
  return request<API.BaseResponsePageUser>('/admin/users/list', {
    method: 'GET',
    params: {
      pageNum: params.pageNum,
      pageSize: params.pageSize,
      // 后端若后续支持筛选可继续扩展
    },
    ...(options || {}),
  })
}

/** 管理端删除用户 DELETE /admin/users/:id */
export async function deleteUser(
  body: API.DeleteRequest,
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse>(`/admin/users/${body.id}`, {
    method: 'DELETE',
    ...(options || {}),
  })
}

/** 管理端升级 VIP POST /admin/users/:id/upgrade-vip */
export async function upgradeUserVip(id: number, options?: { [key: string]: any }) {
  return request<API.BaseResponseUser>(`/admin/users/${id}/upgrade-vip`, {
    method: 'POST',
    ...(options || {}),
  })
}

/** 管理端撤销 VIP POST /admin/users/:id/revoke-vip */
export async function revokeUserVip(id: number, options?: { [key: string]: any }) {
  return request<API.BaseResponseUser>(`/admin/users/${id}/revoke-vip`, {
    method: 'POST',
    ...(options || {}),
  })
}
