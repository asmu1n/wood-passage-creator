// 旧 SpringDoc 地址已废弃。当前 Spec 由 Go swag 生成，文档 UI：http://localhost:8080/docs
// 如需重新生成 client，请改为可读的 openapi/swagger JSON 地址后再跑 openapi2ts。
export default {
  requestLibPath: "import request from '@/request'",
  schemaPath: 'http://localhost:8080/docs', // 占位：Scalar 页，不是 raw JSON；生成前请换成 swagger.json 导出
  serversPath: './src',
}
