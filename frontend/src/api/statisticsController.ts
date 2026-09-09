import request from '@/request'

/** 管理端系统统计 GET /admin/statistics/overview */
export async function getStatistics(options?: { [key: string]: any }) {
  return request<API.BaseResponseStatisticsVO>('/admin/statistics/overview', {
    method: 'GET',
    ...(options || {}),
  })
}
