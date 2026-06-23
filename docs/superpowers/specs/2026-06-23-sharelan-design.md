# ShareLan（局域网共享工具）设计文档

> 极简局域网工具，实现设备自动发现和文本聊天，替代 QQ/微信在局域网内的轻量聊天场景。

---

## 1. 项目定位

**核心理念：** 极简 · 快速可用 · 无账号 · 无服务器依赖（纯局域网）

**MVP 功能：**
1. 局域网设备自动发现（mDNS）
2. 设备间文本聊天（WebSocket）
3. 聊天记录本地存储（SQLite）

---

## 2. 技术栈

| 层 | 技术 | 约束 |
|---|------|------|
| 后端 | Go 1.24+ | 禁止 Node.js/Rust/Java |
| 前端 | Svelte 5 + TypeScript + TailwindCSS | |
| 桌面 | Go 内置 HTTP Server + WebView | 禁止 Electron/Tauri |
| 服务发现 | mDNS (Bonjour/Zeroconf) | |
| 通信 | WebSocket | |
| 存储 | SQLite | |

---

## 3. 架构设计

### 3.1 整体架构

每个设备运行一个 Go 进程，作为本地局域网通信服务。设备间通过 mDNS 发现彼此，建立直接 WebSocket 连接。

```
┌─────────────────────┐         ┌─────────────────────┐
│     设备 A          │         │     设备 B          │
│                     │         │                     │
│  ┌───────────────┐  │         │  ┌───────────────┐  │
│  │  Go 本地服务   │  │         │  │  Go 本地服务   │  │
│  │  :17888       │  │ mDNS    │  │  :17888       │  │
│  │  HTTP + WS    │──┼────────┼─→│  HTTP + WS    │  │
│  └───────┬───────┘  │ 直接 WS │  └───────┬───────┘  │
│          │          │◄───────→│          │          │
│  ┌───────┴───────┐  │         │  ┌───────┴───────┐  │
│  │  WebView      │  │         │  │  WebView      │  │
│  │  (Svelte 5)   │  │         │  │  (Svelte 5)   │  │
│  └───────────────┘  │         │  └───────────────┘  │
└─────────────────────┘         └─────────────────────┘
```

**网络模型：** LAN peer discovery + direct WS connections

- 不是 P2P 网状网络（没有 NAT traversal、full mesh routing）
- 不是中心化服务器架构
- 每个设备是"本地服务"，设备间通过直接 WS 连接通信

### 3.2 Go 后端职责

| 职责 | 说明 |
| --- | --- |
| **mDNS 服务发现** | 广播本机信息，发现在线设备 |
| **WebSocket 消息转发** | 接收前端消息 → 直接 WS 转发给目标设备 |
| **SQLite 消息存储** | 收到消息后写入本地数据库 |
| **HTTP 静态文件服务** | 托管 Svelte 前端编译产物 |
| **WebView 宿主** | 绑定 WebView 窗口打开本地页面 |

Go 不做：NAT 穿透、消息队列、集群协调、设备状态同步。

### 3.3 消息流转

```
设备 A 前端          → ws://127.0.0.1:17888/ws → 设备 A Go
设备 A Go (收到消息)   → 存入本地 SQLite
设备 A Go             → ws://192.168.1.5:17888/ws → 设备 B Go (直接转发)
设备 B Go (收到消息)   → 存入本地 SQLite
设备 B Go             → ws://127.0.0.1:17888/ws → 设备 B 前端
```

**关键简化：** Go 只做 message router，不是双 Hub 转发网络。

---

## 4. 通信设计

### 4.1 mDNS 服务发现

- **服务类型：** `_sharelan._tcp`
- **端口：** 固定 17888（HTTP + WebSocket 共用同一端口）
  - 若端口被占用，依次尝试 17889、17890…（上限 +10）
  - **mDNS 只广播最终成功绑定的端口**，避免 fallback 后出现多端口混乱
  - **端口变化时触发 mDNS re-announce**，重新广播最新端口，覆盖旧缓存
  - **reannounce 需 debounce**（2~3s 内最多一次），避免频繁重启/端口切换时产生大量幽灵设备记录
- **广播内容（TXT records）：**
  ```json
  {
    "id": "uuid",
    "name": "hostname",
    "port": "17888"
  }
  ```
- **设备在线判断：** 不依赖 mDNS TTL/超时
  - **主要依据：** WebSocket 连接状态（对方主动断开 = 下线）
  - **辅助依据：** 每 30s 发送 WS ping/pong 心跳检测"假连接"（WiFi 切换、sleep 唤醒、NAT 重置后连接可能失效但未断开）
  - **兜底：** 超过 N 分钟无任何消息 + 无有效 WS 连接 = 下线
  - mDNS 仅用于设备发现，不用于心跳检测
- **库：** `github.com/grandcat/zeroconf`（Go 生态中最稳定的 mDNS 库，无更好替代）

### 4.2 WebSocket 通信

- **消息结构：**
  ```json
  {
    "id": "uuid",
    "type": "text",
    "from": "device-uuid",
    "to": "device-uuid",
    "content": "消息内容",
    "timestamp": 1719123456789
  }
  ```
- **预留扩展：** `type` 字段未来支持 image / file / clipboard / screen
- **路由规则：** 本机 Go 收到前端消息后，查找目标设备的 WebSocket 连接并转发
- **连接表：** `map[deviceID]*WebSocketConn` — Go 内存中维护当前所有远端设备的 WS 连接
- **连接发起规则：**
  - mDNS 发现新设备后，双方**同时主动发起** WebSocket 连接
  - 连接建立后，通过 `device_id` 比较去重：`device_id` 较小的保留连接，较大的关闭自己的主动连接
  - 保证 A↔B 之间只存在一条 WebSocket 连接
  - **⚠️ 工程级保证（防竞态）：** 连接建立后必须通过 handshake 消息（`type: "handshake"`）确认唯一连接，未完成 handshake 确认的连接不用于消息转发。
    - WS 连接采用**三态模型**：`connecting → handshaking → ready`
    - handshake 流程：
      1. A 连 B，双方各发一条 `{ type: "handshake", from: deviceID }`
      2. 收到对方 handshake 后比较 device_id
      3. device_id 小的保留，大的关闭自己的出站连接
      4. 保留的连接标记为 `ready=true`，开始转发消息
- **心跳检测：** 每 30s 发送 WS ping/pong，连续 3 次无响应判定连接断开
- **重连策略：** WS 断开后，按 `3s → 6s → 12s → 24s → 30s（cap）` 指数退避重连，直到连接成功或设备从 mDNS 中消失

### 4.3 Go 后端代码组织（MVP 极简）

```
backend/
├── main.go      # 入口：初始化、启动
├── mdns.go      # mDNS 广播 + 发现
├── ws.go        # WebSocket: 连接管理 + 消息转发（一个文件搞定 MVP）
├── db.go        # SQLite 初始化 + 消息 CRUD
└── server.go    # HTTP 服务 + 前端静态文件托管
```

**MVP 不分包**，一个功能一个文件。后续按需拆分。

---

## 5. 数据存储

### 5.1 SQLite 表结构

```sql
CREATE TABLE messages (
    id              TEXT PRIMARY KEY,
    type            TEXT NOT NULL DEFAULT 'text',
    from_device     TEXT NOT NULL,
    to_device       TEXT NOT NULL,
    conversation_id TEXT NOT NULL,          -- min(idA, idB) + ":" + max(idA, idB)，统一会话维度
    content         TEXT NOT NULL,
    created_at      INTEGER NOT NULL,           -- Unix timestamp (ms)，统一时区，跨设备排序准确
);

CREATE INDEX idx_messages_conversation ON messages(conversation_id);
CREATE INDEX idx_messages_created      ON messages(created_at);
```

**`conversation_id` 说明：**

- `conversation_id = min(deviceA_id, deviceB_id) + ":" + max(deviceA_id, deviceB_id)`
  - 示例：`"550e8400-e29b-41d4-a716-446655440000:6ba7b810-9dad-11d1-80b4-00c04fd430c8"`
  - 不采用 hash，保持可读可调试
- 用途：未来支持多窗口、会话列表排序
- MVP 阶段即可受益：按 `conversation_id` 查询聊天记录

### 5.2 设备 ID 持久化

首次运行时生成 UUID，写入 `~/.sharelan/config.json`，后续启动读取。

### 5.3 SQLite 文件位置

`~/.sharelan/messages.db`

---

## 6. 桌面方案

- **WebView 库：** `github.com/webview/webview_go`
- **macOS：** 使用系统 WKWebView，零额外依赖
- **Windows：** 依赖 WebView2 Runtime
  - Windows 11 默认自带
  - Windows 10 需安装（首次运行会自动引导安装）
- **行为：** Go 启动 HTTP 服务后，打开 WebView 指向 `http://127.0.0.1:17888`
- **SPA fallback：** HTTP 静态文件服务需处理 SPA 路由，所有非文件路径请求 fallback 到 `index.html`

---

## 7. UI 设计

### 7.1 布局

```
┌──────────────────────────────────────┐
│  ShareLan                            │
├──────────────┬───────────────────────┤
│  在线设备      │  设备名称             │
│  ┌────────┐  │  ┌──────────────────┐ │
│  │ ● 设备A │  │  │  消息1 (对方)     │ │
│  │ ● 设备B │  │  │  消息2 (我方)     │ │
│  │ ○ 设备C │  │  │  消息3 (对方)     │ │
│  └────────┘  │  │  ...             │ │
│              │  └──────────────────┘ │
│              │  ┌──────────────────┐ │
│              │  │ 输入框...        │ │
│              │  │ [发送]           │ │
│              │  └──────────────────┘ │
└──────────────┴───────────────────────┘
```

### 7.2 复制粘贴交互（核心设计目标）

- 所有消息文本使用 `select-text`，禁止 `user-select: none`
- 不劫持 contextmenu 事件，保留浏览器原生右键菜单
- 消息列表使用原生滚动容器，不拦截鼠标选择事件
- 双击选中词、三击选中整行、Shift+点击跨消息选择
- 复制时只复制纯文本内容，不附带时间戳/发送者等 UI 噪音

### 7.3 组件结构

```
App.svelte
├── Sidebar.svelte          # 左侧设备列表
│   └── DeviceItem.svelte   # 设备条目（名称 + 绿色在线指示器）
└── ChatPanel.svelte         # 右侧聊天区域
    ├── ChatHeader.svelte    # 当前聊天对象名称
    ├── MessageList.svelte   # 消息列表（原生滚动，可选中）
    │   └── MessageItem.svelte # 消息气泡（我方蓝色右对齐 / 对方灰色左对齐，文本可选中）
    └── MessageInput.svelte  # 输入框（Enter 发送 / Shift+Enter 换行）
```

### 7.4 状态管理（Svelte 5 runes）

```typescript
// stores/devices.ts
$state: 设备列表 Device[]  // { id, name, ip, port, online }

// stores/messages.ts
$state: 消息列表 Message[]
$derived: 按 conversation_id 分组的消息

// stores/activeChat.ts
$state: 当前选中的设备 id
```

---

## 8. 工程结构（MVP）

```text
ShareLan/
├── backend/
│   ├── go.mod
│   ├── main.go           # 入口：初始化各模块、启动 HTTP/WS 服务、打开 WebView
│   ├── mdns.go           # mDNS 广播本机 + 发现在线设备
│   ├── ws.go             # WebSocket 连接管理 + 消息转发（MVP 一个文件）
│   ├── db.go             # SQLite 初始化 + 消息 CRUD
│   └── server.go         # HTTP Server：托管前端 + WS 路由
│
├── frontend/
│   ├── package.json
│   ├── svelte.config.js
│   ├── vite.config.ts
│   ├── tailwind.config.ts
│   ├── tsconfig.json
│   ├── index.html
│   └── src/
│       ├── main.ts
│       ├── App.svelte
│       ├── components/
│       │   ├── Sidebar.svelte
│       │   ├── DeviceItem.svelte
│       │   ├── ChatPanel.svelte
│       │   ├── MessageList.svelte
│       │   ├── MessageItem.svelte
│       │   └── MessageInput.svelte
│       ├── stores/
│       │   ├── devices.ts
│       │   ├── messages.ts
│       │   └── activeChat.ts
│       ├── lib/
│       │   ├── ws.ts      # WebSocket 客户端封装
│       │   └── types.ts   # 共享类型定义
│       └── app.css
│
├── docs/superpowers/specs/
│   └── 2026-06-23-sharelan-design.md
├── .claude/settings.json
└── README.md
```

---

## 9. 构建与部署

- 前端通过 `npm run build` 输出到 `frontend/dist/`
- Go 后端通过 `//go:embed frontend/dist` 将前端静态文件嵌入二进制
- 最终产物：**单文件可执行二进制**
- 运行方式：直接执行二进制 → 自动启动 HTTP 服务 + 打开 WebView
- 数据目录：`~/.sharelan/`（SQLite 数据库 + 设备配置）

---

## 10. 非目标（明确不实现）

- ❌ 图片发送
- ❌ 文件传输
- ❌ 群聊
- ❌ 登录系统
- ❌ 云同步
- ❌ 表情系统
- ❌ 消息撤回

---

## 11. 扩展预留

消息结构统一为 `{ id, type, payload, timestamp }`，未来可扩展：
- `type: "image"` — 图片消息
- `type: "file"` — 文件传输
- `type: "clipboard"` — 剪贴板共享
- `type: "screen"` — 屏幕截图
