# n2n-admin 后端服务

基于 Go 语言构建的高性能管理后端。

## 技术栈
- **Web 框架**: Gin
- **ORM**: GORM
- **数据库**: SQLite (默认文件名 `n2n_admin.db`)
- **鉴权**: JWT (golang-jwt/v5)
- **密码加密**: Bcrypt

## 目录结构
- `/models`: GORM 数据模型定义 (Node, User, Community, Setting)
- `/utils`: 
    - `n2n_mgmt.go`: 实现 UDP 协议与 n2n 管理端口交互逻辑。
    - `n2n_utils.go`: 包含系统命令执行、MAC/IP 处理、配置文件读写等工具函数。
- `main.go`: 程序入口、API 路由定义、鉴权中间件及静态文件服务逻辑。

## 关键技术点
1. **静态资源嵌入**: 使用 `go:embed` 指令将前端编译产物 (`dist/`) 嵌入二进制，实现单文件分发。
2. **异步日志流**: 使用 Goroutine 监听 `journalctl` 输出，通过 SSE (Server-Sent Events) 实现日志的实时低延迟推送。
3. **活跃中转监测**: 后台任务实时解析转发日志，并在内存中维护 60s 的活跃连接状态。
