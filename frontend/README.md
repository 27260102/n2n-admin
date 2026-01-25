# n2n-admin 前端应用

基于 React 18 打造的现代化管理界面。

## 技术栈
- **框架**: React 18 (TypeScript)
- **构建工具**: Vite
- **UI 组件**: Ant Design 5
- **图标**: Ant Design Icons
- **可视化**: vis-network
- **HTTP 客户端**: Axios (封装了统一鉴权拦截器)

## 目录结构
- `/src/api`: API 统一封装与 Axios 拦截器配置。
- `/src/components`: 通用布局组件 (MainLayout)。
- `/src/pages`: 业务页面（仪表盘、节点管理、社区设置、系统设置、登录）。
- `/src/types`: TypeScript 类型定义。

## 开发规范
1. **统一 API 调用**: 必须使用 `src/api` 导出的封装方法，以确保 JWT Token 自动注入。
2. **鉴权路由**: 使用 `App.tsx` 中的 `ProtectedRoute` 组件保护私有页面。
3. **响应式设计**: 侧边栏支持响应式折叠，兼容移动端查看。