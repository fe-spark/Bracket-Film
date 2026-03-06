# Bracket-Film Web

`web` 是 Bracket-Film 的前端应用，包含前台浏览与管理后台，基于 Next.js 16 + React 19 + Ant Design 6。

## 技术栈

| 类别 | 技术 |
| :--- | :--- |
| 框架 | Next.js 16 (App Router) |
| UI | Ant Design 6 |
| 语言 | TypeScript |
| 请求 | Axios |
| 播放 | Artplayer / hls.js / Video.js |
| 样式 | Less + CSS Modules |

## 目录结构

```text
web/
├─ src/
│  ├─ app/                 # 路由与页面
│  │  ├─ (public)/         # 前台页面
│  │  ├─ login/            # 登录页
│  │  └─ manage/           # 后台页面
│  ├─ components/          # 公共组件
│  ├─ lib/                 # API / 鉴权 / 工具
│  └─ proxy.ts             # /manage 鉴权中间件
├─ next.config.ts          # rewrites 与构建配置
└─ package.json            # 脚本与依赖
```

## 本地开发

### 1) 安装依赖

```bash
npm install
```

### 2) 配置后端地址

在 `web/.env.local` 中配置：

```env
API_URL=http://localhost:3601
```

说明：`next.config.ts` 会将 `/api/*` 重写到 `API_URL/api/*`。

### 3) 启动开发服务

```bash
npm run dev
```

默认访问：`http://localhost:3000`

## 生产构建

```bash
npm run build
npm run start
```

## 说明

- `/manage/*` 路由依赖 `auth-token` Cookie 鉴权。
- 推荐配合根目录 `docker-compose.yml` 进行一体化部署。

## License

MIT License
