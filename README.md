# StudyVideo · 题目讲解视频统一目录站

给运营人员使用的题目讲解视频门户：把公司视频存储的 **MP4 公开直链**统一登记到后台，用户通过固定网址按 **主题 → 分类 → 视频** 三级结构浏览、搜索、在线播放。

## 目录结构

```
StudyVideo/
├── backend/                        # Go 后端（独立 Go module）
│   ├── cmd/studyvideo/main.go      # 程序入口：组装依赖、启动 HTTP 服务
│   ├── internal/
│   │   ├── config/                 # 环境变量配置
│   │   ├── store/                  # MySQL 数据访问 + schema.sql + 模型
│   │   ├── auth/                   # 管理端登录与会话签名
│   │   ├── risk/                   # IP 风控：滑动窗口限流、封禁、真实 IP 解析
│   │   ├── health/                 # MP4 链接健康检查（HEAD/Range 探测）
│   │   ├── httpapi/                # HTTP 路由、公开/管理接口、批量解析
│   │   ├── alert/                  # 告警日志 + Webhook 推送
│   │   └── web/                    # 内嵌前端产物（dist，由 frontend 构建）
│   └── go.mod / go.sum
├── frontend/                       # Vue 3 + Vite + Element Plus 前端（独立项目）
│   ├── src/
│   │   ├── views/                  # 首页、主题、播放、搜索
│   │   ├── views/admin/            # 管理后台（主题/视频/批量/健康/风控）
│   │   ├── components/ api.js router.js utils.js styles.css
│   ├── package.json / vite.config.js
├── deploy/studyvideo.service       # systemd 示例
├── bin/                            # 编译产物（单二进制）
├── .env.example                    # 环境变量模板；复制为 .env 使用
├── Makefile                        # make build / run / test
└── run.sh                          # 加载 .env 并启动二进制
```

前后端完全分离：前端是独立 npm 项目，构建产物输出到 `backend/internal/web/dist`，由 Go `embed` 打进单二进制；后端也可脱离前端独立开发（`go run ./cmd/studyvideo` 会返回“前端未构建”占位页）。

## 核心设计（务必了解）

- **服务端不碰视频字节**：不存储、不转码、不代理、不缓存视频。用户浏览器直连公司 CDN（示例：`https://static.hetaoimg.com/...mp4`），服务器只保存标题、分类、链接等元数据，带宽成本为 0。
- **PDF 配套资料由本站存储**：PDF 体积小、需要在线预览与下载，由运营在后台直接上传，文件保存在服务器 `DATA_DIR` 目录，数据库只存文件路径与元信息；支持 Range 请求（浏览器 PDF 阅读器可翻页），删除资料/视频/主题会同步清理磁盘文件。
- **只支持人工录入**：无平台 API，后台支持“复制 MP4 直链 → 粘贴入库”，并支持批量粘贴。
- **移动端优先**：原生 `<video controls playsinline preload="metadata">`，可拖进度条、可全屏；微信内点击播放（微信禁止自动播放）。
- **风控**：按 IP 统计“播放取链接”请求，单日超过 800 次或短时间高频请求会被临时封禁，后台可查看/解封。

## 快速开始

前置：Go 1.22+、MySQL 8；仅当需要重新构建前端时才需要 Node 18+。

```bash
# 1. 安装前端依赖（仅首次构建需要，使用国内镜像加速）
make deps

# 2. 构建前端并编译后端（产物 bin/studyvideo，前端已内嵌）
make build

# 3. 准备配置
cp .env.example .env
#    编辑 .env：至少修改 DB_DSN、ADMIN_PASSWORD、SESSION_SECRET

# 4. 启动（默认监听 :8080）
./run.sh
```

打开 `http://服务器IP:8080/` 即用户端；`http://服务器IP:8080/admin` 为管理后台（默认密码见 `.env`）。

> 数据库无需手工建表：服务启动时会自动执行 `CREATE DATABASE IF NOT EXISTS` 与建表语句。
> 若 MySQL 账号没有建库权限，请先手工执行：

```sql
CREATE DATABASE studyvideo CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'studyvideo'@'localhost' IDENTIFIED BY '你的强密码';
GRANT ALL PRIVILEGES ON studyvideo.* TO 'studyvideo'@'localhost';
FLUSH PRIVILEGES;
```

## 配置项（环境变量）

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | 监听地址 |
| `DB_DSN` | `studyvideo:studyvideo123@tcp(127.0.0.1:3306)/studyvideo?...` | MySQL 连接串 |
| `ADMIN_PASSWORD` | `admin123` | 管理后台密码，**必须修改** |
| `SESSION_SECRET` | 随机生成 | 登录 Cookie 签名密钥；固定后重启不掉线 |
| `TRUST_PROXY` | `auto` | `auto`：仅当请求来自本机回环时信任 `X-Forwarded-For`；`true` 始终信任；`false` 不信任 |
| `COOKIE_SECURE` | `false` | 全站 HTTPS 时置 `true` |
| `DAILY_VIDEO_LIMIT` | `800` | 单 IP 每日播放请求上限，超出封禁至次日 0 点 |
| `DAILY_TOTAL_LIMIT` | `8000` | 单 IP 每日总 API 请求上限 |
| `BURST_VIDEO_LIMIT` | `20` | 短时间播放请求上限 |
| `BURST_TOTAL_LIMIT` | `120` | 短时间总请求上限 |
| `BURST_WINDOW_SECONDS` | `10` | 短时间统计窗口 |
| `BLOCK_MINUTES` | `30` | 突发超限后的封禁时长 |
| `CHECK_INTERVAL_MINUTES` | `360` | 链接健康检查间隔（分钟） |
| `CHECK_ON_START` | `true` | 启动后自动检查一次链接 |
| `CHECK_TIMEOUT_SECONDS` | `15` | 单条链接探测超时 |
| `CHECK_WORKERS` | `6` | 链接探测并发数 |
| `ALERT_WEBHOOK_URL` | 空 | 可选。失效/风控告警推送到企业微信、钉钉等群机器人（text 消息格式） |
| `DATA_DIR` | `./data` | PDF 配套资料存储目录（**需纳入备份**） |
| `MAX_PDF_SIZE_MB` | `50` | 单个 PDF 大小上限（MB） |
| `STATS_KEEP_DAYS` | `730` | 访问/观看明细保留天数（0 = 永久保留） |

## 管理后台功能

- **概览**：主题数、视频数、失效/未检测数、今日播放与访问量、封禁 IP 数。
- **主题管理**：创建「GESP 真题讲解」「CSP 真题讲解」「GESP 考点讲解」等主题，支持简介与排序（排序值越大越靠前）。
- **分类管理**：在主题下创建分类（如「2024 年 3 月真题」「一级选择题」）。用户进入主题先看到分类，进入分类才是视频列表。删除分类时可选「保留视频（移入未分类）」或「连同视频与资料一起删除」。
- **视频管理**：增删改查、筛选（可按主题 + 分类两级过滤）、单条链接立即检测、复制链接；链接变化后自动重置为“未检测”并重新探测。备注/简介支持 **Markdown 语法**（后台所见即所得编辑器，前台渲染为 HTML）。列表展示每个视频的**访问 / 观看**数据。
- **访问与观看统计**：两个指标均为**独立访客（UV，按 IP + 天去重）**，鼠标悬停可看总次数（PV）与观看转化率。
  - **访问次数**：用户打开视频详情页即计一次（同一天同一 IP 只算 1 个访客，刷新不重复计）
  - **观看次数**：用户点击播放、视频真正开始播放才计（同一天同一 IP 只算 1 个访客，暂停后继续播放不重复计）
  - 转化率 = 观看 UV / 访问 UV，用于判断标题吸引力与实际内容消费情况
  - 明细默认保留 730 天（`STATS_KEEP_DAYS`），删除视频/主题会级联清理统计
- **配套资料（PDF）**：在视频行的「资料」入口中上传/替换/删除 PDF，支持设置名称与排序；文件由本站服务器存储，用户端提供「在线预览」与「下载」，删除视频/主题会级联清理文件。
- **批量录入**：一次粘贴多行，支持以下格式：

  ```
  https://static.hetaoimg.com/crmFiles/abc.mp4
  标题 | https://static.hetaoimg.com/crmFiles/abc.mp4
  标题 | 标签1,标签2 | https://static.hetaoimg.com/crmFiles/abc.mp4
  标题 | 标签1,标签2 | https://static.hetaoimg.com/crmFiles/abc.mp4 | 备注
  ```

  纯链接行会以文件名作为标题；重复链接自动跳过；`#` 开头的行忽略。
- **链接健康**：查看状态（有效/失效/未检测）、实时进度条、失效原因（404/403/401/超时/返回网页等）、一键全面检测、单条重新检测。定时任务按 `CHECK_INTERVAL_MINUTES` 自动执行，失效会记录日志并在配置 Webhook 时告警。
- **访问风控**：当前策略、封禁中的 IP（可解封）、今日访问排行。风控计数说明见下。

## 风控逻辑说明

服务端看不到 CDN 上的视频请求，因此统计的是 **`GET /api/videos/{id}/play`（播放页获取直链）** 的次数，这也是普通用户触发视频访问的必经入口：

1. 每次请求按 `IP + 当天` 累加计数（MySQL `ip_daily` 表）。
2. 短时间高频（默认 10 秒 > 20 次播放，或 > 120 次总请求）→ 封禁 30 分钟。
3. 单日播放请求 > 800 次（或总请求 > 8000 次）→ 封禁至次日 0 点。
4. 封禁写入 `blocked_ips`，公开接口返回 `429` + `Retry-After`，管理端不受影响。
5. 管理员登录同样有限制（默认 10 次 / 10 分钟）。

真实 IP 获取：同机 nginx 反代时 `TRUST_PROXY=auto` 即可正确识别 `X-Forwarded-For`；若前面是云负载均衡（非同机），需设 `TRUST_PROXY=true` 并确保 LB 会重写 XFF 头。

## 性能与容量参考

- **视频字节不经过本站**：播放时浏览器直连公司 CDN，服务器带宽只用于首屏静态资源、API JSON 与 PDF 下载。
- **内置 gzip 压缩**：HTML/CSS/JS/JSON/SVG 自动 gzip（小于 512B 或 PDF/图片等已压缩格式跳过，Range 请求不压缩）。首屏资源约 **1547KB → 431KB**，20Mbps 带宽下首屏由 0.63s 降到 0.18s，并发承载提升约 3.6 倍。
- **单机实测**（8 核，含 MySQL 读写）：首页 API 约 7300 QPS（P99 16ms）、播放取链接约 14500 QPS、静态资源约 6 万 QPS；进程常驻内存约 28MB。
- **4C4G / 20Mbps 估算**：日常几百人同时在线无压力；瓶颈在首屏带宽（约 5-6 人/秒的首次访问）而非 CPU/数据库。PDF 下载量大时建议控制单文件大小或改走 CDN。
- 若前置 nginx，可再开 `gzip_static on;` 与静态资源缓存进一步减负。

## 反向代理（nginx 示例）

```nginx
server {
    listen 80;
    server_name study.example.com;
    client_max_body_size 4m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_http_version 1.1;
    }
}
```

HTTPS 场景：证书配置好后设 `COOKIE_SECURE=true`。注意页面是 HTTPS、视频直链是 HTTP 时会被浏览器拦截（混合内容），请确认 MP4 链接本身是 HTTPS。

## systemd 部署示例

见 `deploy/studyvideo.service`，复制到 `/etc/systemd/system/` 后：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now studyvideo
```

## 开发

```bash
# 终端 1：后端（:8080）
cd backend && go run ./cmd/studyvideo     # 或根目录 make run（需已构建过前端）

# 终端 2：前端热更新（:5173，自动代理 /api 到 8080）
cd frontend && npm run dev
```

重新构建后前端会重新嵌入二进制，直接 `make build` 即可。

## 测试

```bash
make test      # 单元测试（不依赖数据库）

# 含 MySQL 集成测试
TEST_DB_DSN='studyvideo:密码@tcp(127.0.0.1:3306)/studyvideo?parseTime=true&charset=utf8mb4&loc=Local' make test
```

## 常见问题

- **视频打不开/失效**：到「链接健康」查看具体原因。403/401 通常意味着公司开启了防盗链或改为签名地址，需要与公司确认发布方式；本系统只能提前发现，无法绕过。
- **播放页能开但视频转圈**：检查该直链是否支持 Range（可拖进度条）以及是否 HTTPS；可先用浏览器直接打开 MP4 直链验证。
- **微信里不自动播放**：微信限制，必须用户点击播放器，页面已给出提示。
- **修改了前端但页面没变**：需要 `make build` 重新构建（前端资源已编译进二进制）。
- **内容对外发布授权**：请与公司确认视频链接的对外使用范围。

## 明确不做

用户注册/付费/复杂权限、视频存储/转码/下载、公司平台 API 对接、弹幕评论等社交功能。

> 说明：视频仍全部走外部直链（不占本站存储与带宽）；仅 PDF 配套资料属于本站存储。
