# Frontend (wood-passage-creator)

已从旧项目迁移并对齐 Go 后端 API。

- 开发：`npm install && npm run dev`（默认 http://localhost:5173）
- Vite 代理：`/api`、`/health` → `http://localhost:8080`
- 后端文档：http://localhost:8080/docs
- 主要变更：auth 前缀、`/article/list/self`、SSE named events、VIP mock-session/complete、分页 `total`

---

# 前端项目

基于 Vue 3 + Vite + Ant Design Vue 的前端项目模板。

## 环境要求

- Node.js 22+

## 技术栈

- Vue 3
- Vite
- TypeScript
- Ant Design Vue
- Axios
- Pinia
- Vue Router

## 开发

```bash
# 安装依赖
npm install

# 启动开发服务器
npm run dev

# 构建生产版本
npm run build

# 代码格式化
npm run format

# 代码检查
npm run lint
```

## 生成 API 代码

```bash
npm run openapi2ts
```

## 目录结构

```
src/
├── api/           # API 接口定义
├── assets/        # 静态资源
├── components/    # 公共组件
├── config/        # 配置文件
├── layouts/       # 布局组件
├── pages/         # 页面组件
├── router/        # 路由配置
├── stores/        # Pinia 状态管理
└── utils/         # 工具函数
```
