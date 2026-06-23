# ShareLan MVP 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现一个局域网设备自动发现 + 文本聊天的桌面工具 MVP

**Architecture:** Go 后端作为本地通信服务，提供 HTTP（前端托管）+ WebSocket（消息转发）+ mDNS（设备发现）+ SQLite（消息存储）；Svelte 5 前端通过 WebView 展示。设备间通过 mDNS 发现彼此，建立直接 WS 连接，Go 只做 message router。

**Tech Stack:** Go 1.24+, `github.com/grandcat/zeroconf`, `github.com/coder/websocket`, `modernc.org/sqlite`, `github.com/webview/webview_go`, Svelte 5, TypeScript, TailwindCSS v4, Vite

---

### 前置任务：项目初始化

**Files:**
- Create: `ShareLan/.gitignore`
- Create: `ShareLan/README.md`
- Create: `ShareLan/backend/` (directory)
- Create: `ShareLan/frontend/` (directory)

- [ ] **Step 1: 创建 .gitignore**

```text
# Go
backend/tmp/
backend/dist/

# Frontend
frontend/node_modules/
frontend/dist/

# OS
.DS_Store
Thumbs.db

# IDE
.idea/
.vscode/
*.swp
*.swo
```

- [ ] **Step 2: git init + 初次提交**

```bash
cd /Users/hywel/workspaces/ShareLan
git init
git add .gitignore
git commit -m "chore: 项目初始化"
```

---

### Task 1: Go 模块初始化 + 数据库层

**Files:**
- Create: `backend/go.mod`
- Create: `backend/db.go`

- [ ] **Step 1: 初始化 Go 模块**

```bash
cd /Users/hywel/workspaces/ShareLan/backend
go mod init sharelan
```

- [ ] **Step 2: 安装依赖**

```bash
cd /Users/hywel/workspaces/ShareLan/backend
go get modernc.org/sqlite
go get github.com/google/uuid
```

- [ ] **Step 3: 实现 db.go**

```go
// backend/db.go
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Message struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	FromDevice     string `json:"from"`
	ToDevice       string `json:"to"`
	ConversationID string `json:"conversation_id"`
	Content        string `json:"content"`
	CreatedAt      int64  `json:"timestamp"`
}

func initDB() (*sql.DB, error) {
	dir := filepath.Join(os.Getenv("HOME"), ".sharelan")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	dbPath := filepath.Join(dir, "messages.db")
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	if err := createTables(db); err != nil {
		return nil, err
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id              TEXT PRIMARY KEY,
			type            TEXT NOT NULL DEFAULT 'text',
			from_device     TEXT NOT NULL,
			to_device       TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			content         TEXT NOT NULL,
			created_at      INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
		CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
	`)
	return err
}

func saveMessage(db *sql.DB, msg *Message) error {
	_, err := db.Exec(
		`INSERT OR IGNORE INTO messages (id, type, from_device, to_device, conversation_id, content, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.Type, msg.FromDevice, msg.ToDevice,
		msg.ConversationID, msg.Content, msg.CreatedAt,
	)
	return err
}

func getConversation(db *sql.DB, conversationID string, limit, offset int) ([]Message, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.Query(
		`SELECT id, type, from_device, to_device, conversation_id, content, created_at
		 FROM messages WHERE conversation_id = ?
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		conversationID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Type, &m.FromDevice, &m.ToDevice,
			&m.ConversationID, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	// 反转为升序
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}
```

- [ ] **Step 4: 验证编译**

```bash
cd /Users/hywel/workspaces/ShareLan/backend
go build -o /dev/null .
```

Expected: 编译成功，无输出。

- [ ] **Step 5: 提交**

```bash
cd /Users/hywel/workspaces/ShareLan
git add backend/go.mod backend/go.sum backend/db.go
git commit -m "feat: 添加 SQLite 数据层"
```

---

### Task 2: HTTP 静态文件服务 + SPA fallback

**Files:**
- Create: `backend/server.go`

- [ ] **Step 1: 安装依赖**

```bash
cd /Users/hywel/workspaces/ShareLan/backend
go get github.com/coder/websocket
```

- [ ] **Step 2: 实现 server.go**

```go
// backend/server.go
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

func startHTTPServer(port int, hub *Hub) *http.Server {
	// 嵌入的前端文件系统
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Fatalf("无法加载前端文件: %v", err)
	}

	mux := http.NewServeMux()

	// WebSocket 路由
	mux.HandleFunc("/ws", hub.ServeWS)

	// SPA fallback: 请求路径不是文件路径时返回 index.html
	fileServer := http.FileServer(http.FS(distFS))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 先尝试作为静态文件提供
		path := strings.TrimPrefix(r.URL.Path, "/")
		if _, err := fs.Stat(distFS, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback — 返回 index.html
		index, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	}))

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("HTTP 服务已启动: http://%s", addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务异常: %v", err)
		}
	}()

	return server
}
```

Note: 需要在 `server.go` 顶部加上 `"fmt"` import（已在代码块中包含，但在完整文件中需确保 import 完整）。

- [ ] **Step 3: 添加端口绑定辅助函数**

在 `server.go` 末尾添加端口绑定函数：

```go
func bindPort(startPort int) int {
	for port := startPort; port < startPort+10; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			ln.Close()
			return port
		}
	}
	log.Fatalf("无法绑定端口 (尝试 %d-%d)", startPort, startPort+9)
	return 0
}
```

需要补充 import：`"net"`。

- [ ] **Step 4: 验证编译**

```bash
cd /Users/hywel/workspaces/ShareLan/backend
go build -o /dev/null .
```

Expected: 编译成功。

- [ ] **Step 5: 提交**

```bash
cd /Users/hywel/workspaces/ShareLan
git add backend/server.go
git commit -m "feat: 添加 HTTP 服务 + SPA fallback"
```

---

### Task 3: mDNS 服务发现

**Files:**
- Create: `backend/mdns.go`

- [ ] **Step 1: 安装依赖**

```bash
cd /Users/hywel/workspaces/ShareLan/backend
go get github.com/grandcat/zeroconf
```

- [ ] **Step 2: 实现 mdns.go**

```go
// backend/mdns.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const serviceType = "_sharelan._tcp"
const domain = "local."

var hostname string

func init() {
	var err error
	hostname, err = os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	// 去掉 .local 后缀
	hostname = strings.TrimSuffix(hostname, ".local")
}

// DeviceInfo 描述一个发现的设备
type DeviceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

// MDNSService 管理 mDNS 广播和发现
type MDNSService struct {
	server   *zeroconf.Server
	devices  map[string]*DeviceInfo
	mu       sync.RWMutex
	onFound  func(DeviceInfo)
	onLost   func(string)
	ctx      context.Context
	cancel   context.CancelFunc
}

func startMDNS(deviceID string, port int, onFound func(DeviceInfo), onLost func(string)) (*MDNSService, error) {
	ctx, cancel := context.WithCancel(context.Background())

	s := &MDNSService{
		devices: make(map[string]*DeviceInfo),
		onFound: onFound,
		onLost:  onLost,
		ctx:     ctx,
		cancel:  cancel,
	}

	// 获取本机局域网 IP
	localIP := getLocalIP()

	// 启动广播
	server, err := zeroconf.Register(
		deviceID,                          // 实例名称 = deviceID
		serviceType,                       // 服务类型
		domain,                            // 域名
		port,                              // 端口
		[]string{
			"id=" + deviceID,
			"name=" + hostname,
			fmt.Sprintf("port=%d", port),
		},
		nil,                               // 接口（nil = 所有接口）
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("mDNS 注册失败: %w", err)
	}
	s.server = server

	log.Printf("mDNS 广播已启动: %s (%s) on port %d", hostname, localIP, port)

	// 启动发现
	go s.discover()

	return s, nil
}

// reAnnounce 端口变化时重新广播
func (s *MDNSService) reAnnounce(deviceID string, port int) {
	if s.server != nil {
		s.server.Shutdown()
	}

	server, err := zeroconf.Register(
		deviceID, serviceType, domain, port,
		[]string{
			"id=" + deviceID,
			"name=" + hostname,
			fmt.Sprintf("port=%d", port),
		},
		nil,
	)
	if err != nil {
		log.Printf("mDNS reannounce 失败: %v", err)
		return
	}
	s.server = server
	log.Printf("mDNS 已重新广播，新端口: %d", port)
}

func (s *MDNSService) discover() {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalf("mDNS 解析器创建失败: %v", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for entry := range entries {
			s.handleEntry(entry)
		}
	}()

	err = resolver.Browse(s.ctx, serviceType, domain, entries)
	if err != nil {
		log.Printf("mDNS 浏览失败: %v", err)
	}
}

func (s *MDNSService) handleEntry(entry *zeroconf.ServiceEntry) {
	id := ""
	name := ""
	port := entry.Port

	for _, txt := range entry.Text {
		parts := strings.SplitN(txt, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "id":
			id = parts[1]
		case "name":
			name = parts[1]
		}
	}

	if id == "" || len(entry.AddrIPv4) == 0 {
		return
	}

	// 不添加自己
	if id == entry.Instance {
		return
	}

	ip := entry.AddrIPv4[0].String()

	s.mu.Lock()
	existing, exists := s.devices[id]
	if !exists || existing.Port != port || existing.IP != ip {
		s.devices[id] = &DeviceInfo{ID: id, Name: name, IP: ip, Port: port}
		s.mu.Unlock()
		if s.onFound != nil {
			s.onFound(DeviceInfo{ID: id, Name: name, IP: ip, Port: port})
		}
	} else {
		s.mu.Unlock()
	}
}

func (s *MDNSService) Stop() {
	s.cancel()
	if s.server != nil {
		s.server.Shutdown()
	}
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}
```

- [ ] **Step 3: 验证编译**

```bash
cd /Users/hywel/workspaces/ShareLan/backend
go build -o /dev/null .
```

Expected: 编译成功。

- [ ] **Step 4: 提交**

```bash
cd /Users/hywel/workspaces/ShareLan
git add backend/mdns.go
git commit -m "feat: 添加 mDNS 服务发现"
```

---

### Task 4: WebSocket Hub（核心模块）

**Files:**
- Create: `backend/ws.go`

**设计说明：**
- 三态连接模型：`connecting → handshaking → ready`
- 连接表：`map[string]*peerConn`（key = deviceID）
- 每 30s ping/pong 心跳
- 断线指数退避重连：3s → 6s → 12s → 24s → 30s（cap）
- handshake 去重防竞态

- [ ] **Step 1: 实现 ws.go**

```go
// backend/ws.go
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
)

// 连接状态
type connState int

const (
	stateConnecting  connState = iota // 刚刚建立 TCP 连接
	stateHandshaking                  // 正在 handshake 确认
	stateReady                        // handshake 完成，可转发消息
)

const (
	pingInterval   = 30 * time.Second
	pongTimeout    = 10 * time.Second
	maxRetryDelay  = 30 * time.Second
	initialRetry   = 3 * time.Second
)

// peerConn 表示到一个远端设备的 WebSocket 连接
type peerConn struct {
	deviceID string
	conn     *websocket.Conn
	state    connState
	mu       sync.Mutex
}

// Hub 管理所有 WebSocket 连接
type Hub struct {
	db        *sql.DB
	deviceID  string
	deviceName string
	localPort int

	peers   map[string]*peerConn // deviceID -> conn
	peersMu sync.RWMutex

	// 本机前端连接（只保留一个）
	localConn   *websocket.Conn
	localConnMu sync.Mutex

	// 设备发现回调 — mDNS 发现设备后，Hub 发起连接
	connectToPeer func(deviceID, ip string, port int)
}

// WSMessage 统一消息结构
type WSMessage struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	From      string `json:"from"`
	To        string `json:"to"`
	Content   string `json:"content,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

func newHub(db *sql.DB, deviceID, deviceName string, localPort int) *Hub {
	return &Hub{
		db:         db,
		deviceID:   deviceID,
		deviceName: deviceName,
		localPort:  localPort,
		peers:      make(map[string]*peerConn),
	}
}

// ServeWS 处理来自本机前端的 WebSocket 连接
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // 局域网内不需要验证 Origin
	})
	if err != nil {
		log.Printf("WebSocket 接受失败: %v", err)
		return
	}

	// 只保留一个前端连接
	h.localConnMu.Lock()
	if h.localConn != nil {
		h.localConn.Close(websocket.StatusNormalClosure, "新连接接入")
	}
	h.localConn = conn
	h.localConnMu.Unlock()

	log.Println("前端 WebSocket 已连接")
	h.handleLocalConnection(conn)
}

// handleLocalConnection 处理前端发来的消息
func (h *Hub) handleLocalConnection(conn *websocket.Conn) {
	defer func() {
		h.localConnMu.Lock()
		if h.localConn == conn {
			h.localConn = nil
		}
		h.localConnMu.Unlock()
		conn.Close(websocket.StatusNormalClosure, "连接关闭")
		log.Println("前端 WebSocket 已断开")
	}()

	ctx := context.Background()

	for {
		var msg WSMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return
			}
			log.Printf("读取前端消息失败: %v", err)
			return
		}

		h.handleMessage(&msg)
	}
}

// handleMessage 处理收到的消息并转发
func (h *Hub) handleMessage(msg *WSMessage) {
	// 填充消息字段
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.From == "" {
		msg.From = h.deviceID
	}
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().UnixMilli()
	}

	// 计算 conversation_id
	cid := conversationID(msg.From, msg.To)

	// 存入本地 SQLite
	dbMsg := &Message{
		ID:             msg.ID,
		Type:           msg.Type,
		FromDevice:     msg.From,
		ToDevice:       msg.To,
		ConversationID: cid,
		Content:        msg.Content,
		CreatedAt:      msg.Timestamp,
	}
	if err := saveMessage(h.db, dbMsg); err != nil {
		log.Printf("保存消息失败: %v", err)
	}

	// 推送到本机前端（回显）
	h.pushToLocal(msg)

	// 转发给目标设备
	h.forwardToPeer(msg)
}

// forwardToPeer 转发消息给目标设备
func (h *Hub) forwardToPeer(msg *WSMessage) {
	if msg.To == "" {
		return
	}

	h.peersMu.RLock()
	pc, ok := h.peers[msg.To]
	h.peersMu.RUnlock()

	if !ok || pc == nil {
		log.Printf("目标设备 %s 不在线，消息 %s 无法转发", msg.To, msg.ID)
		return
	}

	pc.mu.Lock()
	ready := pc.state == stateReady
	conn := pc.conn
	pc.mu.Unlock()

	if !ready || conn == nil {
		log.Printf("目标设备 %s 连接未就绪", msg.To)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := wsjson.Write(ctx, conn, msg); err != nil {
		log.Printf("转发消息到 %s 失败: %v", msg.To, err)
	}
}

// pushToLocal 推送消息到本机前端
func (h *Hub) pushToLocal(msg *WSMessage) {
	h.localConnMu.Lock()
	conn := h.localConn
	h.localConnMu.Unlock()

	if conn == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := wsjson.Write(ctx, conn, msg); err != nil {
		log.Printf("推送到前端失败: %v", err)
	}
}

// ConnectToPeer 发起对远端设备的 WebSocket 连接
func (h *Hub) ConnectToPeer(deviceID, ip string, port int) {
	h.peersMu.RLock()
	existing, ok := h.peers[deviceID]
	h.peersMu.RUnlock()

	if ok && existing != nil {
		existing.mu.Lock()
		state := existing.state
		existing.mu.Unlock()
		if state == stateReady || state == stateHandshaking {
			return // 已有有效连接
		}
	}

	go h.connectWithRetry(deviceID, ip, port)
}

func (h *Hub) connectWithRetry(deviceID, ip string, port int) {
	delay := initialRetry

	for {
		conn, err := h.tryConnect(deviceID, ip, port)
		if err == nil {
			// 开始 handshake
			h.startHandshake(deviceID, conn, true) // true = 我们是主动发起方
			return
		}

		log.Printf("连接 %s@%s:%d 失败: %v，%v 后重试", deviceID, ip, port, err, delay)
		time.Sleep(delay)

		delay = delay * 2
		if delay > maxRetryDelay {
			delay = maxRetryDelay
		}
	}
}

func (h *Hub) tryConnect(deviceID, ip string, port int) (*websocket.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("ws://%s:%d/ws", ip, port)
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	// 存储连接（state = connecting）
	pc := &peerConn{
		deviceID: deviceID,
		conn:     conn,
		state:    stateConnecting,
	}
	h.peersMu.Lock()
	// 如果已有同设备连接且状态更高，关闭新建的
	existing, exists := h.peers[deviceID]
	if exists && existing.state >= stateHandshaking {
		h.peersMu.Unlock()
		conn.Close(websocket.StatusNormalClosure, "重复连接")
		return nil, fmt.Errorf("已有更高状态的连接")
	}
	h.peers[deviceID] = pc
	h.peersMu.Unlock()

	// 启动读取 goroutine（用于接收远端消息 + handshake 响应）
	go h.readPeer(deviceID, conn)

	return conn, nil
}

// startHandshake 发送 handshake 消息
func (h *Hub) startHandshake(deviceID string, conn *websocket.Conn, initiator bool) {
	pc := h.getPeerConn(deviceID)
	if pc == nil {
		return
	}

	pc.mu.Lock()
	pc.state = stateHandshaking
	pc.mu.Unlock()

	// 发送 handshake
	hs := WSMessage{
		Type:      "handshake",
		From:      h.deviceID,
		To:        deviceID,
		Content:   h.deviceName,
		Timestamp: time.Now().UnixMilli(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := wsjson.Write(ctx, conn, &hs); err != nil {
		log.Printf("发送 handshake 到 %s 失败: %v", deviceID, err)
		h.removePeer(deviceID)
		return
	}

	// 如果是主动发起方，等待对端关闭或确认
	if initiator {
		time.Sleep(500 * time.Millisecond) // 给对端时间响应

		pc.mu.Lock()
		if pc.state == stateHandshaking {
			// 如果还是 handshaking，说明对端可能也连了我们，检查 device_id
			if h.deviceID > deviceID {
				// 我们 device_id 大，应该关闭连接
				log.Printf("device_id %s > %s，关闭主动连接", h.deviceID, deviceID)
				pc.state = stateConnecting // 标记为不活跃
				pc.mu.Unlock()
				conn.Close(websocket.StatusNormalClosure, "device_id 更大，让连接留给对端")
				h.removePeer(deviceID)
				return
			}
			// 我们 device_id 小，保留连接
			pc.state = stateReady
			pc.mu.Unlock()
			log.Printf("与 %s 的 WebSocket 连接已就绪 (主动发起)", deviceID)
		} else {
			pc.mu.Unlock()
		}
	}
}

// readPeer 持续读取远端设备的消息
func (h *Hub) readPeer(deviceID string, conn *websocket.Conn) {
	defer func() {
		// 连接意外断开，重连
		h.removePeer(deviceID)
		h.reconnectPeer(deviceID)
	}()

	for {
		var msg WSMessage
		if err := wsjson.Read(context.Background(), conn, &msg); err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return
			}
			log.Printf("读取 %s 消息失败: %v", deviceID, err)
			return
		}

		switch msg.Type {
		case "handshake":
			h.handleHandshake(deviceID, &msg)
		case "text":
			// 存储 + 推送到本机前端
			h.handleMessage(&msg)
		}
	}
}

// handleHandshake 处理收到的 handshake 消息
func (h *Hub) handleHandshake(fromDevice string, msg *WSMessage) {
	pc := h.getPeerConn(fromDevice)
	if pc == nil {
		return
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.state == stateReady {
		return // 已经 ready
	}

	// 收到 handshake 时比较 device_id
	if h.deviceID < msg.From {
		// 本机 device_id 小，保留连接
		pc.state = stateReady
		log.Printf("与 %s (%s) 的 WebSocket 连接已就绪 (handshake 确认)", fromDevice, msg.Content)
	} else {
		// 本机 device_id 大，关闭连接
		log.Printf("device_id %s > %s，由对端保留连接，本机关闭", h.deviceID, msg.From)
		pc.conn.Close(websocket.StatusNormalClosure, "对端保留连接")
		pc.state = stateConnecting // 标记无效
	}
}

// HandleIncomingConnection 处理远端设备主动连入（由 HTTP handler 调用）
func (h *Hub) HandleIncomingConnection(deviceID string, conn *websocket.Conn) {
	// 检查是否已有连接
	h.peersMu.Lock()
	existing, exists := h.peers[deviceID]
	if exists && existing.state >= stateHandshaking {
		h.peersMu.Unlock()
		conn.Close(websocket.StatusNormalClosure, "已有连接")
		return
	}

	pc := &peerConn{
		deviceID: deviceID,
		conn:     conn,
		state:    stateConnecting,
	}
	h.peers[deviceID] = pc
	h.peersMu.Unlock()

	// 开始 handshake（非发起方，等待对方发 handshake）
	go h.readPeer(deviceID, conn)
}

// removePeer 移除对端连接
func (h *Hub) removePeer(deviceID string) {
	h.peersMu.Lock()
	delete(h.peers, deviceID)
	h.peersMu.Unlock()
}

// reconnectPeer 尝试重连（指数退避）
func (h *Hub) reconnectPeer(deviceID string) {
	// 从 mDNS 设备表中获取 IP 和端口
	// 这个回调通过外部注册实现
}

func (h *Hub) getPeerConn(deviceID string) *peerConn {
	h.peersMu.RLock()
	defer h.peersMu.RUnlock()
	return h.peers[deviceID]
}

func conversationID(a, b string) string {
	if a < b {
		return a + ":" + b
	}
	return b + ":" + a
}

// 保持 Ping/Pong 心跳
func (h *Hub) startPingLoop() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for range ticker.C {
		h.peersMu.RLock()
		for deviceID, pc := range h.peers {
			if pc.state != stateReady {
				continue
			}
			go func(id string, conn *websocket.Conn) {
				ctx, cancel := context.WithTimeout(context.Background(), pongTimeout)
				defer cancel()
				if err := conn.Ping(ctx); err != nil {
					log.Printf("%s ping 失败: %v", id, err)
					h.removePeer(id)
					h.reconnectPeer(id)
				}
			}(deviceID, pc.conn)
		}
		h.peersMu.RUnlock()
	}
}
```

- [ ] **Step 2: 验证编译**

```bash
cd /Users/hywel/workspaces/ShareLan/backend
go build -o /dev/null .
```

Expected: 编译成功。

- [ ] **Step 3: 提交**

```bash
cd /Users/hywel/workspaces/ShareLan
git add backend/ws.go
git commit -m "feat: 添加 WebSocket Hub（三态连接 + handshake + 心跳 + 重连）"
```

---

### Task 5: 主入口 main.go

**Files:**
- Create: `backend/main.go`
- Modify: `backend/server.go`（添加 ws handler）

- [ ] **Step 1: 实现 main.go**

```go
// backend/main.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/webview/webview_go"
)

type Config struct {
	DeviceID string `json:"device_id"`
}

func loadOrGenerateDeviceID() string {
	dir := filepath.Join(os.Getenv("HOME"), ".sharelan")
	os.MkdirAll(dir, 0755)
	cfgPath := filepath.Join(dir, "config.json")

	var cfg Config
	data, err := os.ReadFile(cfgPath)
	if err == nil {
		json.Unmarshal(data, &cfg)
		if cfg.DeviceID != "" {
			return cfg.DeviceID
		}
	}

	cfg.DeviceID = uuid.New().String()
	data, _ = json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)
	return cfg.DeviceID
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	// 1. 加载设备 ID
	deviceID := loadOrGenerateDeviceID()
	log.Printf("设备 ID: %s", deviceID)
	log.Printf("主机名: %s", hostname)

	// 2. 初始化数据库
	db, err := initDB()
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer db.Close()
	log.Println("数据库已初始化")

	// 3. 绑定端口
	port := bindPort(17888)
	log.Printf("端口已绑定: %d", port)

	// 4. 创建 Hub
	hub := newHub(db, deviceID, hostname, port)

	// 5. Hub 启动心跳
	go hub.startPingLoop()

	// 6. 启动 HTTP 服务（此时 frontend/dist 必须存在）
	//    注意：第一次编译前需要先构建前端，否则 go:embed 会失败
	httpServer := startHTTPServer(port, hub)
	defer httpServer.Close()

	// 7. 启动 mDNS
	mdns, err := startMDNS(deviceID, port,
		// onFound: 新设备发现时发起 WS 连接
		func(d DeviceInfo) {
			log.Printf("发现设备: %s (%s:%d)", d.Name, d.IP, d.Port)
			hub.ConnectToPeer(d.ID, d.IP, d.Port)
		},
		// onLost: 设备下线
		func(deviceID string) {
			log.Printf("设备下线: %s", deviceID)
		},
	)
	if err != nil {
		log.Fatalf("mDNS 启动失败: %v", err)
	}
	defer mdns.Stop()
	log.Println("mDNS 服务已启动")

	// 8. WebView 优雅退出
	go func() {
		time.Sleep(500 * time.Millisecond)
		log.Println("正在打开 WebView...")
	}()

	// 9. 打开 WebView
	w := webview.New(true)
	defer w.Destroy()
	w.SetTitle("ShareLan")
	w.SetSize(900, 640, webview.HintNone)
	w.Navigate(fmt.Sprintf("http://127.0.0.1:%d", port))
	w.Run()
}
```

- [ ] **Step 2: 安装 WebView 依赖**

```bash
cd /Users/hywel/workspaces/ShareLan/backend
go get github.com/webview/webview_go
```

- [ ] **Step 3: 主入口 server.go 添加 WS 升级处理**

需要修改 `server.go`，让 `/ws` 路由既能处理本地前端连接，也能处理远端设备连接。在 `startHTTPServer` 函数中，`/ws` handler 需要区分本地和远端：

对于 MVP，`/ws` 端点的简单处理：本地和远端都通过同一个 Hub.ServeWS 入口。实际需要在 Hub 中区分。让我们修改方案——`/ws` 端点统一处理，Hub 内部通过检查 `r.Host` 或连接后的第一条消息区分。

更简单的方式：本机前端走 `/ws`，远端设备也走 `/ws`。Hub 接受所有连接，通过第一条消息的 `type` 区分（`handshake` = 远端设备，`text` = 本机前端）。

修改 `ws.go` 中 `ServeWS`：

```go
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("WebSocket 接受失败: %v", err)
		return
	}

	// 读取第一条消息来判断是前端还是远端
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var msg WSMessage
	if err := wsjson.Read(ctx, conn, &msg); err != nil {
		conn.Close(websocket.StatusNormalClosure, "读取首条消息失败")
		return
	}

	if msg.Type == "handshake" {
		// 远端设备连入
		deviceID := msg.From
		log.Printf("远端设备连入: %s (%s)", deviceID, msg.Content)
		h.HandleIncomingConnection(deviceID, conn)
	} else if msg.Type == "hello" {
		// 本机前端连入
		h.localConnMu.Lock()
		if h.localConn != nil {
			h.localConn.Close(websocket.StatusNormalClosure, "新连接接入")
		}
		h.localConn = conn
		h.localConnMu.Unlock()
		log.Println("前端 WebSocket 已连接")
		go h.handleLocalConnection(conn)
	}
}
```

同时需要修改前端 ws.ts，连接后先发一条 `{ type: "hello", from: deviceID }`。

- [ ] **Step 4: 验证编译（注意：此时 frontend/dist 还不存在，需要先创建占位）**

```bash
mkdir -p /Users/hywel/workspaces/ShareLan/frontend/dist
touch /Users/hywel/workspaces/ShareLan/frontend/dist/index.html
cd /Users/hywel/workspaces/ShareLan/backend
go build -o /dev/null .
```

Expected: 编译成功。

- [ ] **Step 5: 提交**

```bash
cd /Users/hywel/workspaces/ShareLan
git add backend/main.go backend/go.mod backend/go.sum
git commit -m "feat: 添加主入口 main.go，集成所有模块"
```

---

### Task 6: 前端项目初始化（Svelte 5 + Vite + TailwindCSS）

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/svelte.config.js`
- Create: `frontend/tsconfig.json`
- Create: `frontend/tsconfig.node.json`
- Create: `frontend/tailwind.config.ts`
- Create: `frontend/postcss.config.js`
- Create: `frontend/index.html`
- Create: `frontend/src/app.css`
- Create: `frontend/src/main.ts`
- Create: `frontend/src/vite-env.d.ts`

- [ ] **Step 1: 创建 package.json**

```json
{
  "name": "sharelan-frontend",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "devDependencies": {
    "@sveltejs/vite-plugin-svelte": "^5.0.0",
    "@tsconfig/svelte": "^5.0.0",
    "autoprefixer": "^10.4.20",
    "postcss": "^8.4.49",
    "svelte": "^5.0.0",
    "tailwindcss": "^3.4.17",
    "typescript": "^5.7.0",
    "vite": "^6.0.0"
  }
}
```

- [ ] **Step 2: 安装依赖**

```bash
cd /Users/hywel/workspaces/ShareLan/frontend
npm install
```

- [ ] **Step 3: 创建 vite.config.ts**

```typescript
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
```

- [ ] **Step 4: 创建 svelte.config.js**

```javascript
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

export default {
  preprocess: vitePreprocess(),
};
```

- [ ] **Step 5: 创建 tsconfig.json**

```json
{
  "extends": "@tsconfig/svelte/tsconfig.json",
  "compilerOptions": {
    "target": "ESNext",
    "useDefineForClassFields": true,
    "module": "ESNext",
    "resolveJsonModule": true,
    "allowJs": true,
    "checkJs": true,
    "isolatedModules": true,
    "strict": true,
    "moduleDetection": "force"
  },
  "include": ["src/**/*.ts", "src/**/*.svelte"]
}
```

- [ ] **Step 6: 创建 tailwind 和 postcss 配置**

`postcss.config.js`:
```javascript
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
};
```

`tailwind.config.ts`:
```typescript
import type { Config } from 'tailwindcss';

export default {
  content: ['./index.html', './src/**/*.{svelte,ts,js}'],
  theme: {
    extend: {},
  },
  plugins: [],
} satisfies Config;
```

- [ ] **Step 7: 创建 index.html**

```html
<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>ShareLan</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

- [ ] **Step 8: 创建入口文件**

`src/app.css`:
```css
@tailwind base;
@tailwind components;
@tailwind utilities;

html, body, #app {
  margin: 0;
  padding: 0;
  height: 100%;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}
```

`src/vite-env.d.ts`:
```typescript
/// <reference types="svelte" />
/// <reference types="vite/client" />
```

`src/main.ts`:
```typescript
import App from './App.svelte';
import { mount } from 'svelte';

const app = mount(App, {
  target: document.getElementById('app')!,
});

export default app;
```

- [ ] **Step 9: 验证构建**

```bash
cd /Users/hywel/workspaces/ShareLan/frontend
npm run build
```

Expected: 构建成功，`dist/` 目录生成。

- [ ] **Step 10: 提交**

```bash
cd /Users/hywel/workspaces/ShareLan
git add frontend/
git commit -m "feat: 初始化 Svelte 5 + Vite + TailwindCSS 前端项目"
```

---

### Task 7: 前端共享类型 + WebSocket 客户端

**Files:**
- Create: `frontend/src/lib/types.ts`
- Create: `frontend/src/lib/ws.ts`

- [ ] **Step 1: 创建 types.ts**

```typescript
// 设备信息
export interface Device {
  id: string;
  name: string;
  ip: string;
  port: number;
  online: boolean;
}

// 消息结构
export interface Message {
  id: string;
  type: 'text' | 'handshake' | 'hello';
  from: string;
  to: string;
  content: string;
  timestamp: number;
}

// 应用整体状态
export interface AppState {
  deviceId: string;
  deviceName: string;
}
```

- [ ] **Step 2: 创建 ws.ts**

```typescript
import type { Message } from './types';

type MessageHandler = (msg: Message) => void;
type StatusHandler = (connected: boolean) => void;

export class WSClient {
  private ws: WebSocket | null = null;
  private url: string;
  private deviceId: string;
  private deviceName: string;
  private onMessage: MessageHandler;
  private onStatus: StatusHandler;
  private reconnectDelay = 1000;
  private maxReconnectDelay = 30000;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private destroyed = false;

  constructor(
    url: string,
    deviceId: string,
    deviceName: string,
    onMessage: MessageHandler,
    onStatus: StatusHandler
  ) {
    this.url = url;
    this.deviceId = deviceId;
    this.deviceName = deviceName;
    this.onMessage = onMessage;
    this.onStatus = onStatus;
    this.connect();
  }

  private connect() {
    if (this.destroyed) return;

    this.ws = new WebSocket(this.url);

    this.ws.onopen = () => {
      this.onStatus(true);
      this.reconnectDelay = 1000;
      // 连接后立即发送 hello 消息
      this.send({
        id: crypto.randomUUID(),
        type: 'hello',
        from: this.deviceId,
        to: '',
        content: this.deviceName,
        timestamp: Date.now(),
      });
    };

    this.ws.onclose = () => {
      this.onStatus(false);
      this.scheduleReconnect();
    };

    this.ws.onerror = () => {
      this.ws?.close();
    };

    this.ws.onmessage = (event) => {
      try {
        const msg: Message = JSON.parse(event.data);
        this.onMessage(msg);
      } catch (e) {
        console.error('消息解析失败:', e);
      }
    };
  }

  private scheduleReconnect() {
    if (this.destroyed) return;
    if (this.reconnectTimer) return;

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, this.reconnectDelay);

    this.reconnectDelay = Math.min(
      this.reconnectDelay * 2,
      this.maxReconnectDelay
    );
  }

  send(msg: Message) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  destroy() {
    this.destroyed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
  }
}
```

- [ ] **Step 3: 验证构建**

```bash
cd /Users/hywel/workspaces/ShareLan/frontend
npm run build
```

Expected: 构建成功。

- [ ] **Step 4: 提交**

```bash
cd /Users/hywel/workspaces/ShareLan
git add frontend/src/lib/
git commit -m "feat: 前端 WebSocket 客户端 + 类型定义"
```

---

### Task 8: 前端 Svelte Stores

**Files:**
- Create: `frontend/src/stores/devices.ts`
- Create: `frontend/src/stores/messages.ts`
- Create: `frontend/src/stores/activeChat.ts`

- [ ] **Step 1: 创建 devices store**

```typescript
// src/stores/devices.ts
import { writable } from 'svelte/store';
import type { Device } from '../lib/types';

export const devices = writable<Device[]>([]);

export function upsertDevice(device: Device) {
  devices.update(list => {
    const idx = list.findIndex(d => d.id === device.id);
    if (idx >= 0) {
      list[idx] = device;
      return [...list];
    }
    return [...list, device];
  });
}

export function removeDevice(id: string) {
  devices.update(list => list.filter(d => d.id !== id));
}
```

- [ ] **Step 2: 创建 messages store**

```typescript
// src/stores/messages.ts
import { writable, derived } from 'svelte/store';
import type { Message } from '../lib/types';

export const allMessages = writable<Message[]>([]);

export const messagesByConversation = derived(allMessages, ($msgs) => {
  const map = new Map<string, Message[]>();
  for (const msg of $msgs) {
    const convId = conversationId(msg.from, msg.to);
    const list = map.get(convId) || [];
    list.push(msg);
    map.set(convId, list);
  }
  return map;
});

export function addMessage(msg: Message) {
  allMessages.update(list => [...list, msg]);
}

export function loadMessages(msgs: Message[]) {
  allMessages.set(msgs);
}

function conversationId(a: string, b: string): string {
  return a < b ? `${a}:${b}` : `${b}:${a}`;
}
```

- [ ] **Step 3: 创建 activeChat store**

```typescript
// src/stores/activeChat.ts
import { writable } from 'svelte/store';

export const activeDeviceId = writable<string | null>(null);
```

- [ ] **Step 4: 验证构建**

```bash
cd /Users/hywel/workspaces/ShareLan/frontend
npm run build
```

Expected: 构建成功。

- [ ] **Step 5: 提交**

```bash
cd /Users/hywel/workspaces/ShareLan
git add frontend/src/stores/
git commit -m "feat: 前端 Svelte stores（设备、消息、活跃会话）"
```

---

### Task 9: 前端 UI 组件

**Files:**
- Create: `frontend/src/App.svelte`
- Create: `frontend/src/components/Sidebar.svelte`
- Create: `frontend/src/components/DeviceItem.svelte`
- Create: `frontend/src/components/ChatPanel.svelte`
- Create: `frontend/src/components/MessageList.svelte`
- Create: `frontend/src/components/MessageItem.svelte`
- Create: `frontend/src/components/MessageInput.svelte`

- [ ] **Step 1: 创建 App.svelte**

```svelte
<!-- src/App.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from './components/Sidebar.svelte';
  import ChatPanel from './components/ChatPanel.svelte';
  import { WSClient } from './lib/ws';
  import { addMessage, loadMessages } from './stores/messages';
  import { upsertDevice, removeDevice } from './stores/devices';
  import { activeDeviceId } from './stores/activeChat';
  import type { Message, Device } from './lib/types';

  let wsClient: WSClient | null = null;
  let connected = $state(false);

  // 从 URL 获取设备信息（未来来自 mDNS）
  const deviceId = crypto.randomUUID();
  const deviceName = '本机';

  onMount(() => {
    wsClient = new WSClient(
      `ws://127.0.0.1:17888/ws`,
      deviceId,
      deviceName,
      handleMessage,
      (status) => { connected = status; }
    );

    return () => {
      wsClient?.destroy();
    };
  });

  function handleMessage(msg: Message) {
    if (msg.type === 'text') {
      addMessage(msg);
    }
  }

  function sendMessage(content: string) {
    const targetId = activeDeviceId.value;
    if (!targetId || !wsClient) return;

    const msg: Message = {
      id: crypto.randomUUID(),
      type: 'text',
      from: deviceId,
      to: targetId,
      content,
      timestamp: Date.now(),
    };

    wsClient.send(msg);
    addMessage(msg);
  }

  function handleDeviceSelect(id: string) {
    activeDeviceId.set(id);
  }
</script>

<div class="flex h-screen bg-white">
  <div class="w-60 flex-shrink-0 border-r border-gray-200">
    <Sidebar
      onSelect={handleDeviceSelect}
      onDeviceFound={(d) => upsertDevice(d)}
      onDeviceLost={(id) => removeDevice(id)}
    />
  </div>
  <div class="flex-1 flex flex-col">
    {#if connected}
      <ChatPanel {sendMessage} />
    {:else}
      <div class="flex-1 flex items-center justify-center text-gray-400">
        <div class="text-center">
          <p class="text-lg">正在连接...</p>
          <p class="text-sm mt-2">请确保后端服务已启动</p>
        </div>
      </div>
    {/if}
  </div>
</div>
```

- [ ] **Step 2: 创建 Sidebar.svelte**

```svelte
<!-- src/components/Sidebar.svelte -->
<script lang="ts">
  import { devices } from '../stores/devices';
  import { activeDeviceId } from '../stores/activeChat';
  import DeviceItem from './DeviceItem.svelte';

  let { onSelect, onDeviceFound, onDeviceLost } = $props();

  // Sidebar 模拟设备数据——实际应由 WS/mDNS 驱动
  // 为了 MVP 验证，留空由后端推送

  $effect(() => {
    // dev 环境下打印设备变化
    console.log('设备列表:', $devices);
  });
</script>

<div class="h-full flex flex-col">
  <div class="p-3 border-b border-gray-200">
    <h1 class="text-base font-semibold text-gray-800">ShareLan</h1>
  </div>
  <div class="flex-1 overflow-y-auto">
    <div class="px-2 py-1 text-xs text-gray-400 uppercase tracking-wide">
      在线设备
    </div>
    {#each $devices as device (device.id)}
      <DeviceItem
        {device}
        isActive={$activeDeviceId === device.id}
        onclick={() => onSelect(device.id)}
      />
    {:else}
      <div class="px-3 py-4 text-sm text-gray-400 text-center">
        未发现设备
      </div>
    {/each}
  </div>
</div>
```

- [ ] **Step 3: 创建 DeviceItem.svelte**

```svelte
<!-- src/components/DeviceItem.svelte -->
<script lang="ts">
  import type { Device } from '../lib/types';

  let { device, isActive, onclick }: {
    device: Device;
    isActive: boolean;
    onclick: () => void;
  } = $props();
</script>

<button
  onclick={onclick}
  class="w-full flex items-center gap-2 px-3 py-2 text-left text-sm hover:bg-gray-100 transition-colors {isActive ? 'bg-blue-50 border-l-2 border-blue-500' : ''}"
>
  <span class="w-2 h-2 rounded-full flex-shrink-0 {device.online ? 'bg-green-500' : 'bg-gray-300'}"></span>
  <span class="truncate">{device.name}</span>
</button>
```

- [ ] **Step 4: 创建 ChatPanel.svelte**

```svelte
<!-- src/components/ChatPanel.svelte -->
<script lang="ts">
  import { activeDeviceId } from '../stores/activeChat';
  import { devices } from '../stores/devices';
  import MessageList from './MessageList.svelte';
  import MessageInput from './MessageInput.svelte';

  let { sendMessage }: { sendMessage: (content: string) => void } = $props();

  let currentDevice = $derived.by(() => {
    return $devices.find(d => d.id === $activeDeviceId) ?? null;
  });
</script>

{#if currentDevice}
  <div class="flex flex-col h-full">
    <div class="px-4 py-3 border-b border-gray-200 bg-gray-50">
      <h2 class="text-sm font-medium text-gray-800">{currentDevice.name}</h2>
    </div>
    <div class="flex-1 overflow-y-auto px-4 py-3">
      <MessageList deviceId={currentDevice.id} />
    </div>
    <div class="border-t border-gray-200">
      <MessageInput {sendMessage} />
    </div>
  </div>
{:else}
  <div class="flex-1 flex items-center justify-center text-gray-400">
    <p>请选择一个设备开始聊天</p>
  </div>
{/if}
```

- [ ] **Step 5: 创建 MessageList.svelte**

```svelte
<!-- src/components/MessageList.svelte -->
<script lang="ts">
  import { activeDeviceId } from '../stores/activeChat';
  import { allMessages } from '../stores/messages';
  import MessageItem from './MessageItem.svelte';
  import type { Message } from '../lib/types';

  let { deviceId: _deviceId }: { deviceId: string } = $props();

  let filteredMessages = $derived.by(() => {
    const myId = ''; // TODO: 从实际设备 ID 获取
    // 简单过滤：显示当前活跃会话的消息
    return $allMessages.filter(m => {
      const myDeviceId = $activeDeviceId || '';
      const convId = conversationId(myDeviceId, _deviceId);
      const msgConvId = conversationId(m.from, m.to);
      return msgConvId === convId;
    });
  });

  function conversationId(a: string, b: string): string {
    return a < b ? `${a}:${b}` : `${b}:${a}`;
  }

  let container: HTMLDivElement;
  $effect(() => {
    // 新消息时滚动到底部
    if (container) {
      container.scrollTop = container.scrollHeight;
    }
  });
</script>

<div bind:this={container} class="h-full overflow-y-auto select-text">
  {#each filteredMessages as msg (msg.id)}
    <MessageItem message={msg} />
  {:else}
    <div class="flex items-center justify-center h-full text-gray-400 text-sm">
      暂无消息，发送第一条消息吧
    </div>
  {/each}
</div>
```

- [ ] **Step 6: 创建 MessageItem.svelte**

```svelte
<!-- src/components/MessageItem.svelte -->
<script lang="ts">
  import type { Message } from '../lib/types';

  let { message, isMine = false }: { message: Message; isMine?: boolean } = $props();

  let timeStr = $derived.by(() => {
    const d = new Date(message.timestamp);
    return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
  });
</script>

<div class="flex {isMine ? 'justify-end' : 'justify-start'} mb-3">
  <div
    class="max-w-[70%] px-3 py-2 rounded-lg {isMine ? 'bg-blue-500 text-white' : 'bg-gray-100 text-gray-800'}"
  >
    <p class="whitespace-pre-wrap break-words select-text">{message.content}</p>
    <p class="text-xs mt-1 opacity-60 {isMine ? 'text-right' : 'text-left'}">
      {timeStr}
    </p>
  </div>
</div>
```

- [ ] **Step 7: 创建 MessageInput.svelte**

```svelte
<!-- src/components/MessageInput.svelte -->
<script lang="ts">
  import { activeDeviceId } from '../stores/activeChat';

  let { sendMessage }: { sendMessage: (content: string) => void } = $props();

  let text = $state('');

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  }

  function send() {
    const content = text.trim();
    if (!content || !$activeDeviceId) return;
    sendMessage(content);
    text = '';
  }
</script>

<div class="flex items-end gap-2 p-3 bg-white">
  <textarea
    bind:value={text}
    onkeydown={handleKeydown}
    placeholder="输入消息... (Enter 发送, Shift+Enter 换行)"
    rows="3"
    class="flex-1 resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:border-blue-400"
  ></textarea>
  <button
    onclick={send}
    disabled={!text.trim() || !$activeDeviceId}
    class="px-4 py-2 rounded-lg bg-blue-500 text-white text-sm font-medium hover:bg-blue-600 disabled:bg-gray-300 disabled:cursor-not-allowed"
  >
    发送
  </button>
</div>
```

- [ ] **Step 8: 验证构建**

```bash
cd /Users/hywel/workspaces/ShareLan/frontend
npm run build
```

Expected: 构建成功，`frontend/dist/` 生成完整前端。

- [ ] **Step 9: 提交**

```bash
cd /Users/hywel/workspaces/ShareLan
git add frontend/src/
git commit -m "feat: 前端 UI 组件（完整聊天界面）"
```

---

### Task 10: 编译集成 + 端到端验证

**Files:**
- Modify: `backend/main.go`（版本确认）
- Test: 端到端运行测试

- [ ] **Step 1: 确保前端已构建**

```bash
cd /Users/hywel/workspaces/ShareLan/frontend
npm run build
ls dist/
```

Expected: `dist/` 下包含 `index.html`、`assets/` 等文件。

- [ ] **Step 2: 编译 Go 后端**

```bash
cd /Users/hywel/workspaces/ShareLan/backend
CGO_ENABLED=1 go build -o sharelan .
```

Expected: 在当前目录生成 `sharelan` 可执行文件。

- [ ] **Step 3: 运行并验证**

```bash
cd /Users/hywel/workspaces/ShareLan/backend
./sharelan
```

Expected:
- HTTP 服务启动，日志显示端口
- WebView 窗口打开，显示 ShareLan UI
- 界面显示"正在连接..."然后转为聊天界面

注意：运行 `sharelan` 会打开 WebView，需要在桌面环境执行。在 headless 环境可以先注释 WebView 部分，用浏览器打开 `http://127.0.0.1:17888` 测试。

- [ ] **Step 4: 提交**

```bash
cd /Users/hywel/workspaces/ShareLan
git add -A
git commit -m "feat: 编译集成 + 端到端可用"
```

---

## 实现注意事项

### WS 连接三态模型（防竞态关键）

```
connecting → handshaking → ready
```

每个阶段严格的进入/退出条件，Claude Code 必须围绕三态编写所有连接管理代码。

### mDNS 注意事项

- `zeroconf.Register()` 返回前服务已在广播，无需额外启动延迟
- Browse 是阻塞的，需要在 goroutine 中运行
- 发现自己的设备通过比较 Instance 来排除

### Go embed 条件编译

`//go:embed all:frontend/dist` 要求 `frontend/dist` 目录**在编译时存在**。构建前必须先 `cd frontend && npm run build`。

### CGO 要求

WebView 需要 `CGO_ENABLED=1`。macOS 上需要 Xcode Command Line Tools。

### 跨设备测试

在同一局域网的两台机器上分别运行编译后的二进制，等待 mDNS 发现后即可互发消息。

---

## 计划自审修正（关键缺口修复）

自审发现以下缺口必须在实现时修正：

### 1. 后端须将设备事件通过 WS 推送到前端

在 `ws.go` 的 `Hub` 中添加两个方法：

```go
// pushDeviceOnline 推设备上线事件到前端
func (h *Hub) pushDeviceOnline(deviceID, name, ip string, port int) {
	h.localConnMu.Lock()
	conn := h.localConn
	h.localConnMu.Unlock()
	if conn == nil {
		return
	}
	msg := WSMessage{
		ID:        uuid.New().String(),
		Type:      "device_online",
		From:      deviceID,
		Content:   fmt.Sprintf(`{"name":"%s","ip":"%s","port":%d}`, name, ip, port),
		Timestamp: time.Now().UnixMilli(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	wsjson.Write(ctx, conn, &msg)
}

// pushDeviceOffline 推设备下线事件到前端
func (h *Hub) pushDeviceOffline(deviceID string) {
	// ...类似结构，type = "device_offline"，Content = deviceID
}
```

在 `main.go` 中连接 mDNS 回调与 Hub：

```go
mdns, err := startMDNS(deviceID, port,
	func(d DeviceInfo) {
		log.Printf("发现设备: %s (%s:%d)", d.Name, d.IP, d.Port)
		hub.ConnectToPeer(d.ID, d.IP, d.Port)
		hub.pushDeviceOnline(d.ID, d.Name, d.IP, d.Port) // ← 推送到前端
	},
	func(deviceID string) {
		log.Printf("设备下线: %s", deviceID)
		hub.pushDeviceOffline(deviceID) // ← 推送到前端
	},
)
```

### 2. 前端 types.ts 增加设备事件类型

```typescript
// 消息结构中 type 增加: 'device_online' | 'device_offline'

// 前端设备 ID 需要持久化
export function generateDeviceId(): string {
  let id = localStorage.getItem('sharelan_device_id');
  if (!id) {
    id = crypto.randomUUID();
    localStorage.setItem('sharelan_device_id', id);
  }
  return id;
}
```

### 3. 前端 ws.ts 处理 device_online/device_offline

在 `handleMessage` 中增加：

```typescript
if (msg.type === 'device_online') {
  const data = JSON.parse(msg.content);
  upsertDevice({ id: msg.from, name: data.name, ip: data.ip, port: data.port, online: true });
} else if (msg.type === 'device_offline') {
  const device = devicesList.find(d => d.id === msg.content);
  if (device) upsertDevice({ ...device, online: false });
}
```

### 4. App.svelte 设备 ID 使用 localStorage 持久化

```typescript
const deviceId = $state(localStorage.getItem('sharelan_device_id') || (() => {
  const id = crypto.randomUUID();
  localStorage.setItem('sharelan_device_id', id);
  return id;
})());
```

### 5. MessageList.svelte 获取本机设备 ID 以正确标识 isMine

```svelte
<script lang="ts">
  // 从 localStorage 读取本机 ID
  let myDeviceId = $state(localStorage.getItem('sharelan_device_id') || '');
  // ...
</script>
{#each filteredMessages as msg (msg.id)}
  <MessageItem message={msg} isMine={msg.from === myDeviceId} />
{/each}
```

### 6. Sidebar.svelte 移除无用的 onDeviceFound/onDeviceLost props

只需从 `devices` store 读取数据，无需回调 props。设备数据全部由后端 WS 推送驱动。

