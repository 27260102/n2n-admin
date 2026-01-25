# n2n-admin Web UI

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://golang.org)
[![React Version](https://img.shields.io/badge/React-18-61DAFB.svg)](https://reactjs.org)

**n2n-admin** 是一个专为 [n2n](https://github.com/ntop/n2n) (Layer 2 P2P VPN) 设计的现代化全栈管理面板。它提供了一个直观的 Web 界面，用于管理超级节点（Supernode）、监控边缘节点（Edge）状态以及分析网络拓扑。

---

## ✨ 功能亮点

- 🛡️ **安全先行**: 完整的 JWT 身份验证系统，支持管理员密码在线修改。
- 📊 **网络拓扑可视化**: 实时渲染全网节点连接关系图（基于 vis-network）。
- 🕵️ **实时中转监控**: 深度解析日志，精准识别哪些节点正在走 Relay 模式及转发流量。
- 🗺️ **地理位置识别**: 集成 GeoIP 接口，自动分析在线节点的公网归属地及 ISP 运营商。
- ⚙️ **服务管理**: 在线修改 Supernode 配置（-p, -t, -c 等），支持一键重启系统服务。
- 🛠️ **网络工具箱**: 直接在网页端对虚拟网 IP 进行实时 Ping 和 Traceroute 测试。
- 📝 **实时日志流**: 使用 SSE (Server-Sent Events) 技术实时追踪超级节点运行日志。
- 📦 **单文件部署**: 后端自动嵌入前端静态资源，编译后仅需一个二进制文件。

---

## 🚀 快速开始

### 1. 运行环境准备
- **操作系统**: Linux (Ubuntu/CentOS/Debian 等，需支持 `systemctl`)
- **n2n 组件**: 
  - 请确保已安装 n2n v3.x。
  - **官方下载地址**: [n2n GitHub Releases](https://github.com/ntop/n2n/releases)
- **编译依赖**: 
  - Go 1.21+
  - Node.js 18+

### 2. 编译与安装
```bash
# 克隆仓库
git clone https://github.com/27260102/n2n-admin.git
cd n2n-admin

# 一键构建 (自动处理前后端编译与静态资源嵌入)
chmod +x build.sh
./build.sh
```

### 3. 运行服务
```bash
cd backend
./n2n_admin
```
*注：你可以通过环境变量修改 JWT 密钥：`export N2N_ADMIN_SECRET="your-secret-key"`*

### 4. 访问
打开浏览器访问 `http://your-ip:8080`
- **默认账号**: `admin`
- **默认密码**: `admin123`

---

## 🏗️ 项目架构说明

### 后端 (Backend)
- **核心框架**: [Gin Web Framework](https://gin-gonic.com/)
- **数据库**: SQLite + [GORM](https://gorm.io/)
- **网络通信**: 
  - 通过 UDP 与 n2n 管理接口通信。
  - 通过 `journalctl` 管道实时抓取服务日志。
- **关键模块**:
  - `utils/n2n_mgmt.go`: 封装了对 supernode 管理端口的交互命令。
  - `main.go`: 包含 JWT 中间件、日志流处理器及各功能接口。

### 前端 (Frontend)
- **技术栈**: React 18 + TypeScript + Vite
- **UI 组件库**: [Ant Design 5.x](https://ant.design/)
- **状态管理与路由**: React Router v6 + Axios (带有统一鉴权拦截器)
- **可视化**:
  - `vis-network`: 负责动态拓扑图的渲染。
  - `dayjs`: 处理时间格式化。

---

## 🤝 贡献
欢迎提交 Issue 或 Pull Request 来完善这个项目！

## 📄 开源协议
本项目采用 [MIT License](LICENSE) 协议。