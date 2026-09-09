import request from '@/request'

/** 获取文章详情 GET /article/:taskId */
export async function getArticle(
  params: API.getArticleParams,
  options?: { [key: string]: any },
) {
  const { taskId, ...queryParams } = params
  return request<API.BaseResponseArticle>(`/article/${taskId}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** AI 修改大纲 POST /article/modify-outline */
export async function aiModifyOutline(
  body: API.ArticleAiModifyOutlineRequest,
  options?: { [key: string]: any },
) {
  return request<API.BaseResponseListOutlineSection>('/article/modify-outline', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    data: body,
    ...(options || {}),
  })
}

/** 确认大纲 POST /article/confirm-outline */
export async function confirmOutline(
  body: API.ArticleConfirmOutlineRequest,
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse>('/article/confirm-outline', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    data: body,
    ...(options || {}),
  })
}

/** 确认标题 POST /article/confirm-title */
export async function confirmTitle(
  body: API.ArticleConfirmTitleRequest,
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse>('/article/confirm-title', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    data: body,
    ...(options || {}),
  })
}

/** 创建文章任务 POST /article/create — data 为 taskId string */
export async function createArticle(
  body: API.ArticleCreateRequest,
  options?: { [key: string]: any },
) {
  return request<API.BaseResponseString>('/article/create', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    data: body,
    ...(options || {}),
  })
}

/** 删除文章 POST /article/delete?id= */
export async function deleteArticle(
  body: API.DeleteRequest,
  options?: { [key: string]: any },
) {
  return request<API.BaseResponse>('/article/delete', {
    method: 'POST',
    params: { id: body.id },
    ...(options || {}),
  })
}

/** 获取任务执行日志 GET /article/execution-logs/:taskId */
export async function getExecutionLogs(
  params: API.getExecutionLogsParams,
  options?: { [key: string]: any },
) {
  const { taskId, ...queryParams } = params
  return request<API.BaseResponseAgentExecutionStats>(`/article/execution-logs/${taskId}`, {
    method: 'GET',
    params: { ...queryParams },
    ...(options || {}),
  })
}

/** 我的文章列表 GET /article/list/self */
export async function listArticle(
  params: API.ArticleQueryRequest = {},
  options?: { [key: string]: any },
) {
  return request<API.BaseResponsePageArticle>('/article/list/self', {
    method: 'GET',
    params: {
      pageNum: params.pageNum,
      pageSize: params.pageSize,
      status: params.status,
    },
    ...(options || {}),
  })
}

/** 管理端文章列表 GET /admin/article/list */
export async function listArticleAdmin(
  params: API.ArticleQueryRequest = {},
  options?: { [key: string]: any },
) {
  return request<API.BaseResponsePageArticle>('/admin/article/list', {
    method: 'GET',
    params: {
      pageNum: params.pageNum,
      pageSize: params.pageSize,
      status: params.status,
    },
    ...(options || {}),
  })
}
