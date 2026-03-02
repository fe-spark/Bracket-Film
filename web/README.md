# Bracket Film Web

## 🌐 简介

`web` 是 Bracket-Film 项目的前端展示与管理系统，基于 **Next.js 15+** 与 **Ant Design 6** 构建。它提供了一个高颜值、响应式的影视资源展示界面，以及一个功能丰富、操作简便的管理后台。

### 核心特性
- **优雅的展示层**：基于 Ant Design 深度定制的影视列表、详情页和搜索界面。
- **全能播放器**：集成 `Artplayer` 与 `Video.js`，支持 HLS (.m3u8) 协议及移动端全屏优化。
- **管理后台**：内置完整的影视资源、分类、爬虫任务及系统参数管理界面。
- **现代架构**：使用 Next.js App Router 架构，支持 SSR/ISR 优化 SEO 与首屏加载速度。
- **响应式设计**：完美适配 PC、平板与移动端。

---

## 🛠️ 技术栈

| 类别 | 技术 | 说明 |
| :--- | :--- | :--- |
| **基础框架** | [Next.js 15](https://nextjs.org/) | React 框架，支持 App Router |
| **UI 组件库** | [Ant Design 6](https://ant.design/) | 业界领先的 React UI 库 |
| **视频播放器** | [Artplayer](https://artplayer.org/) / [Video.js](https://videojs.com/) | 支持多协议、高度可定制的播放引擎 |
| **数据请求** | [Axios](https://axios-http.com/) | 简单易用的 HTTP 客户端 |
| **样式方案** | Less / CSS Modules | 兼顾灵活与隔离的样式处理 |
| **开发语言** | [TypeScript](https://www.typescriptlang.org/) | 强类型带来的开发稳定性 |

---

## 📂 项目结构

```text
web
├─ src
│  ├─ app          # Next.js App Router 核心路由
│  │  ├─ (public)  # 前台展示页面
│  │  ├─ manage    # 管理后台页面
│  │  └─ api       # 前端 API Proxy 或边缘逻辑
│  ├─ components   # 通用组件与业务组件
│  ├─ theme        # Ant Design 主题定制
│  └─ public       # 静态资源 (Logo, Icons)
├─ next.config.ts  # Next.js 配置
└─ package.json    # 项目依赖与脚本
```

---

## 🚀 快速开始

### 1. 安装依赖
```bash
npm install
# 或使用 pnpm / yarn
```

### 2. 环境变量配置
在根目录创建 `.env.local` 文件，配置后端 API 地址：
```env
NEXT_PUBLIC_API_URL=http://localhost:8088
```

### 3. 开发环境运行
```bash
npm run dev
```

### 4. 生产环境构建
```bash
npm run build
npm start
```

---

## 📄 许可证

本项目遵循 [MIT License](../LICENSE)。
