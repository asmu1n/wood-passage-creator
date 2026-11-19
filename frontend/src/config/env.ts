/**
 * 前端运行时配置（可按环境覆盖）。
 * 开发态配合 vite proxy：浏览器请求同源 /api → 后端 :8080
 */
export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api'
