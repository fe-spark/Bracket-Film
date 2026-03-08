# 🌐 Bracket-Film Web (Next.js Frontend)

本目录包含 **Bracket-Film** 的前端实现。前端基于 **Next.js 15 + React 19** 构建，提供极致流畅的影视浏览、深度联动筛选及全功能管理后台。

---

## 🚀 技术栈全景

| 类别 | 选用技术 | 核心价值 |
| :--- | :--- | :--- |
| **框架** | **Next.js 15 (App Router)** | 服务端渲染 (SSR) 与极速路由转换 |
| **UI 库** | **Ant Design 6** | 企业级组件库，支持动态主题与令牌系统 |
| **视图** | **React 19** | 新一代渲染引擎，并发模式支持 |
| **播放器** | **Artplayer / HLS.js** | 全能播放引擎，支持倍速、弹幕与跨端对齐 |
| **状态管理** | **Zustand / SWR** | 轻量级状态同步与数据缓存 |
| **样式** | **Less + CSS Modules** | 隔离样式，支持响应式布局 |

---

## 📂 源码目录结构

```text
web/
├── src/
│   ├── app/                # 路由定义 (Pages & Layouts)
│   │   ├── (public)/       # 前台聚合浏览页面
│   │   ├── login/          # 管理员登录系统
│   │   └── manage/         # 响应式管理后台 (Films, Sources, Cron)
│   ├── components/         # 高复用业务组件 (Player, FacetedFilter)
│   ├── lib/                # 基础设施 (Axios, Auth Utils, Constants)
│   └── styles/             # 全局设计变量与 Token
├── next.config.ts          # API 重写 (Rewrites) 与构建优化
└── tailwind.config.ts      # (可选) 样式辅助扩展
```

---

## 🛠️ 本地开发指南

### 1. 安装依赖
```bash
npx pnpm install  # 推荐使用 pnpm
# 或者
npm install
```

### 2. 配置后端 API
在根目录或 `web/` 下创建 `.env.local`：
```env
API_URL=http://localhost:3601
```
> **注意**：`next.config.ts` 已配置反向代理，所有 `/api/*` 的请求会自动指向 `API_URL`。

### 3. 开启开发模式
```bash
npm run dev
```
访问：`http://localhost:3000`

---

## 🏗️ 生产构建 (Production)

```bash
npm run build
npm run start
```
构建产物会进行分包优化与静态资源压缩，建议在生产环境开启 `sharp` 以优化图片载入。

---

## ✨ 核心前端功能
- **Faceted Search UI**：在搜索页面实时反馈各维度的可选状态，极速过滤海量内容。
- **Responsive Management**：完美适配手机与电脑的后台管理界面，随时随地控制系统。
- **Smart Autocomplete**：基于主站搜索索引的智能联想，提升用户查找效率。

---

## 📜 更多文档
- [主从一致性机制 FAQ](../README-FAQ.md)
- [服务器后端文档](../server/README.md)
- [Docker 一键部署](../README-Docker.md)
